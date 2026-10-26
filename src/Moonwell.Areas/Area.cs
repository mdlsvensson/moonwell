using System;
using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell
{
    /// <summary>
    /// A part of the map that units enter, stay in and leave: a game component whose constructor sets <c>Shape</c> (and
    /// <c>Tick</c>), and whose reactions the map overrides. The game tells the area when a unit crosses its edge; a unit
    /// counts as inside while it is alive and has a place (not hidden, not loaded into a transport), by its centre. Two
    /// traps: the game sees the area in whole 32 by 32 cells, so <c>Contains</c> (the true shape) and the area can
    /// disagree within a cell of an edge; and the game sends no event for a unit hidden, shown, loaded or unloaded, so
    /// the package checks those on its own timer every quarter of a second, and such a unit leaves and enters up to
    /// 0.25 seconds late. A unit that stays hidden or loaded inside an area, and a dead one (which leaves when it dies, and
    /// enters again if something brings it back to life there, a hero's revive or a Resurrection), keeps the package's
    /// timer stepping (about 0.3 ms a second) until it leaves, decays or is removed, or the area ends. Add it in the map's <c>Components</c> or with
    /// <c>Game.Instance.Add</c>; ending it lets every unit inside leave.
    /// </summary>
    public abstract class Area : GameComponent
    {
        /// <summary>The shape; null until the constructor sets it.</summary>
        private AreaShape? shape;

        /// <summary>The seconds between OnUnitStays, as set.</summary>
        private float tick;

        /// <summary>The game's region, made when the area is added; null before, and after the area ends.</summary>
        internal region? handle;

        /// <summary>The package's tracker the area was added to; null until it is added.</summary>
        internal AreaTracker? tracker;

        /// <summary>Whether the area is ending or has ended, so that nothing more enters or runs.</summary>
        internal bool stopped;

        /// <summary>The units inside, in the order they entered.</summary>
        private readonly List<Unit> inside = new List<Unit>();

        /// <summary>The game time each unit inside entered; a lookup only, the order is the list's.</summary>
        private readonly Dictionary<Unit, float> since = new Dictionary<Unit, float>();

        /// <summary>The units that left because they have no place (hidden, loaded or dead), in the order they left: the place check enters them again once they have a place and are inside the region.</summary>
        private readonly List<Unit> away = new List<Unit>();

        /// <summary>How many package steps a Tick is, worked out when the area is added; 0 for none.</summary>
        internal int tickSteps;

        /// <summary>The package step the area's first unit entered at; its OnUnitStays are due a Tick of steps after it, and again at every Tick.</summary>
        internal int startStep;

        /// <summary>Whether the area counts for the package timer: it has a unit inside, or one away.</summary>
        private bool active;

        /// <summary>The shape of the area. Set it in the constructor; it is fixed once the area is added, and an area added without one is reported as an error and watches nothing.</summary>
        protected AreaShape Shape
        {
            get
            {
                if (shape == null)
                {
                    throw new InvalidOperationException(NoShape);
                }

                return shape;
            }

            set
            {
                if (added)
                {
                    throw new InvalidOperationException("The Shape of an area is fixed once it is added.");
                }

                shape = value;
            }
        }

#if DEBUG
        /// <summary>The shape, for the debug mode's drawing of the area's outline.</summary>
        internal AreaShape ShapeOfArea()
        {
            return Shape;
        }

#endif
        /// <summary>The seconds between <c>OnUnitStays</c> for each unit inside, rounded up to whole steps of the package's timer (1/32 second, so 1 is 32 steps and 0.04 is 2); 0, the default, and <c>OnUnitStays</c> never runs. Set it in the constructor; it is fixed once the area is added.</summary>
        protected float Tick
        {
            get
            {
                return tick;
            }

            set
            {
                if (added)
                {
                    throw new InvalidOperationException("The Tick of an area is fixed once it is added.");
                }

                tick = value;
            }
        }

        /// <summary>Whether a unit that comes inside enters: false and it does not (and is asked again the next time it comes inside). True here.</summary>
        /// <param name="unit">The unit that came inside.</param>
        public virtual bool Accepts(Unit unit)
        {
            return true;
        }

        /// <summary>Runs when an accepted unit comes inside, and for each unit already inside when the area is added (just after, in Moonwell's order of units).</summary>
        /// <param name="enteringUnit">The unit that entered.</param>
        public virtual void OnUnitEntered(Unit enteringUnit)
        {
        }

        /// <summary>Runs every <c>Tick</c> seconds for each unit inside, in the order they entered, counted from the area's first unit entering.</summary>
        /// <param name="stayingUnit">A unit inside.</param>
        public virtual void OnUnitStays(Unit stayingUnit)
        {
        }

        /// <summary>Runs once when a unit that entered goes out, dies, is hidden or loaded, ends, or the area ends. A unit that dies and is brought back to life inside enters again (<c>OnUnitEntered</c>). The unit is no longer inside here.</summary>
        /// <param name="leavingUnit">The unit that left.</param>
        public virtual void OnUnitLeft(Unit leavingUnit)
        {
        }

        /// <summary>The units inside, in the order they entered; a copy that later changes do not touch.</summary>
        [LocalSafe]
        public IReadOnlyList<Unit> Units
        {
            get
            {
                return new List<Unit>(inside);
            }
        }

        /// <summary>Whether a unit is inside.</summary>
        [LocalSafe]
        public bool Has(Unit unit)
        {
            return since.ContainsKey(unit);
        }

        /// <summary>The game seconds a unit has been inside; 0 when it is not.</summary>
        [LocalSafe]
        public float Since(Unit unit)
        {
            if (since.TryGetValue(unit, out var entered))
            {
                return Runtime.World.Now() - entered;
            }

            return 0f;
        }

        /// <summary>Whether a point is inside the area's true shape, with no call to the game. The game sees the area in 32 cells, so a unit near the edge can be inside the area and not here, or the other way round.</summary>
        [LocalSafe]
        public bool Contains(float x, float y)
        {
            return Shape.Contains(x, y);
        }

        /// <summary>Whether a point is inside the area's true shape; see the other <c>Contains</c>.</summary>
        [LocalSafe]
        public bool Contains(Vector2 point)
        {
            return Shape.Contains(point.X, point.Y);
        }

        /// <summary>The smallest bounds that cover the area's shape.</summary>
        [LocalSafe]
        public Bounds Bounds
        {
            get
            {
                return Shape.Bounds;
            }
        }

        /// <summary>The message of an area without a Shape.</summary>
        private const string NoShape = "An area needs a Shape: set it in its constructor.";

        /// <summary>Makes the area's region and starts watching it.</summary>
        protected sealed override void OnAdded()
        {
            if (shape == null)
            {
                throw new InvalidOperationException(NoShape);
            }

            // A region the map does not have, or a read before the package is installed, fails here, before anything is made.
            _ = shape.Bounds;
            tickSteps = AreaTracker.StepsOf(tick);
            var made = AreaTracker.Current();
            tracker = made;
            made.Begin(this, shape);
        }

        /// <summary>Stops watching, destroys the region, and lets every unit inside leave, in the order they entered.</summary>
        protected sealed override void OnRemoved()
        {
            if (stopped)
            {
                return;
            }

            stopped = true;
            if (tracker != null)
            {
                tracker.Finish(this);
            }

            away.Clear();
            var leaving = new List<Unit>(inside);
            for (var i = 0; i < leaving.Count; i++)
            {
                Exit(leaving[i]);
            }

            Refresh();
        }

        /// <summary>The game says a unit entered the region: it enters the area if it can.</summary>
        internal void HearEnter()
        {
            if (stopped)
            {
                return;
            }

            var unit = Unit.Of(tracker!.World.EventUnit());
            if (unit != null)
            {
                TryEnter(unit);
            }
        }

        /// <summary>The game says a unit left the region.</summary>
        internal void HearLeave()
        {
            if (stopped)
            {
                return;
            }

            var unit = KnownUnits.Find(tracker!.World.EventUnit());
            if (unit != null)
            {
                Drop(unit);
            }
        }

        /// <summary>Enters every unit that is in the region now, once, in Moonwell's order of units; the 0 second timer after the area is added runs it.</summary>
        internal void Sweep()
        {
            if (stopped)
            {
                return;
            }

            var world = tracker!.World;
            var bounds = Shape.Bounds;
            var found = new List<unit>();
            world.UnitsIn(
                MathF.Floor(bounds.Min.X / 32f) * 32f,
                MathF.Floor(bounds.Min.Y / 32f) * 32f,
                MathF.Floor(bounds.Max.X / 32f) * 32f + 32f,
                MathF.Floor(bounds.Max.Y / 32f) * 32f + 32f,
                found);
            var wanted = new OrderedSet<Unit>();
            for (var i = 0; i < found.Count; i++)
            {
                if (world.IsInside(handle!, found[i]))
                {
                    var unit = Unit.Of(found[i]);
                    if (unit != null)
                    {
                        wanted.Add(unit);
                    }
                }
            }

            var live = KnownUnits.Live.Snapshot();
            for (var i = 0; i < live.Count; i++)
            {
                if (stopped)
                {
                    return;
                }

                if (wanted.Contains(live[i]))
                {
                    TryEnter(live[i]);
                }
            }
        }

        /// <summary>Enters a unit that is alive, has a place, is not inside yet and that Accepts takes; false when it did not. A living unit without a place (hidden or loaded) is put among the units away instead, for the place check to enter once it has a place and is inside.</summary>
        internal bool TryEnter(Unit unit)
        {
            if (stopped || since.ContainsKey(unit) || unit.HasEnded || unit.ending)
            {
                return false;
            }

            if (!tracker!.World.HasPlace(unit.Handle))
            {
                if (unit.IsAlive && !away.Contains(unit))
                {
                    away.Add(unit);
                    Refresh();
                }

                return false;
            }

            if (!CallAccepts(unit))
            {
                away.Remove(unit);
                Refresh();
                return false;
            }

            if (stopped || since.ContainsKey(unit) || unit.HasEnded || unit.ending)
            {
                return false;
            }

            since[unit] = Runtime.World.Now();
            inside.Add(unit);
            away.Remove(unit);
            Refresh();
            if (inside.Count == 1)
            {
                startStep = tracker.Steps;
            }

            CallEntered(unit);
            return true;
        }

        /// <summary>Takes a unit out, whatever the reason (it left, died, ended, or has no place), and forgets it was away; OnUnitLeft runs if it was inside.</summary>
        internal void Drop(Unit unit)
        {
            away.Remove(unit);
            Exit(unit);
            Refresh();
        }

        /// <summary>The game says a unit died: it leaves if it was inside, and is kept among the units away while it is in the region, for the place check to enter it again if it lives (a Resurrection sends no event).</summary>
        internal void Die(Unit unit)
        {
            if (stopped || unit.HasEnded || unit.ending)
            {
                return;
            }

            if (since.ContainsKey(unit))
            {
                if (!away.Contains(unit))
                {
                    away.Add(unit);
                }

                Exit(unit);
            }

            Refresh();
        }

        /// <summary>A hero came back to life: it enters again if it is in the region and has a place.</summary>
        internal void Revive(Unit unit)
        {
            if (stopped || since.ContainsKey(unit) || unit.HasEnded || unit.ending)
            {
                return;
            }

            var world = tracker!.World;
            if (world.HasPlace(unit.Handle) && world.IsInside(handle!, unit.Handle))
            {
                TryEnter(unit);
            }
        }

        /// <summary>The place check: takes out each unit that has no place, to enter again, and enters the ones away that have a place and are inside the region.</summary>
        internal void CheckPlaces()
        {
            if (stopped)
            {
                return;
            }

            var world = tracker!.World;
            var present = new List<Unit>(inside);
            for (var i = 0; i < present.Count; i++)
            {
                if (stopped)
                {
                    return;
                }

                var unit = present[i];
                if (since.ContainsKey(unit) && !world.HasPlace(unit.Handle))
                {
                    away.Add(unit);
                    Exit(unit);
                }
            }

            var back = new List<Unit>(away);
            for (var i = 0; i < back.Count; i++)
            {
                if (stopped)
                {
                    return;
                }

                var unit = back[i];
                if (!away.Contains(unit))
                {
                    continue;
                }

                if (unit.HasEnded || unit.ending)
                {
                    away.Remove(unit);
                }
                else if (world.HasPlace(unit.Handle))
                {
                    away.Remove(unit);
                    if (world.IsInside(handle!, unit.Handle))
                    {
                        TryEnter(unit);
                    }
                }
                else if (!unit.IsAlive && !world.IsInside(handle!, unit.Handle))
                {
                    // A corpse that is no longer in the region is forgotten.
                    away.Remove(unit);
                }
            }

            Refresh();
        }

        /// <summary>Runs OnUnitStays for each unit inside, if the area's Tick is due at this package step.</summary>
        internal void Stay(int step)
        {
            if (stopped || tickSteps <= 0 || inside.Count == 0)
            {
                return;
            }

            var passed = step - startStep;
            if (passed <= 0 || passed % tickSteps != 0)
            {
                return;
            }

            var staying = new List<Unit>(inside);
            for (var i = 0; i < staying.Count; i++)
            {
                if (stopped)
                {
                    return;
                }

                if (since.ContainsKey(staying[i]))
                {
                    CallStays(staying[i]);
                }
            }
        }

        /// <summary>Takes a unit out first, then runs OnUnitLeft, once; does nothing when the unit is not inside.</summary>
        private void Exit(Unit unit)
        {
            if (!since.Remove(unit))
            {
                return;
            }

            inside.Remove(unit);
            Refresh();
            CallLeft(unit);
        }

        /// <summary>Tells the tracker when the area starts or stops counting for the package timer: it has a unit inside or away.</summary>
        private void Refresh()
        {
            var now = inside.Count > 0 || away.Count > 0;
            if (now != active)
            {
                active = now;
                tracker!.Activity(now);
            }
        }

        /// <summary>Asks Accepts with the area as the current owner; a failing Accepts is reported and refuses the unit.</summary>
        private bool CallAccepts(Unit unit)
        {
            var previous = Owner.current;
            Owner.current = this;
            var answer = false;
            try
            {
                answer = Accepts(unit);
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(this) + ".Accepts", error);
            }

            Owner.current = previous;
            return answer;
        }

        /// <summary>Runs OnUnitEntered with the area as the current owner; a failing one is reported.</summary>
        private void CallEntered(Unit unit)
        {
            var previous = Owner.current;
            Owner.current = this;
            try
            {
                OnUnitEntered(unit);
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(this) + ".OnUnitEntered", error);
            }

            Owner.current = previous;
        }

        /// <summary>Runs OnUnitStays with the area as the current owner; a failing one is reported.</summary>
        private void CallStays(Unit unit)
        {
            var previous = Owner.current;
            Owner.current = this;
            try
            {
                OnUnitStays(unit);
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(this) + ".OnUnitStays", error);
            }

            Owner.current = previous;
        }

        /// <summary>Runs OnUnitLeft with the area as the current owner; a failing one is reported.</summary>
        private void CallLeft(Unit unit)
        {
            var previous = Owner.current;
            Owner.current = this;
            try
            {
                OnUnitLeft(unit);
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(this) + ".OnUnitLeft", error);
            }

            Owner.current = previous;
        }
    }
}
