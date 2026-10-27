using System;
using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell
{
    /// <summary>A map's own object with a place, for the things the game has no object for: a well, a trap, a missile. It belongs to the owner that made it and ends with it, or by End or Remove. It can tick on its own, in the package's cycle.</summary>
    public abstract class Gizmo : Actor
    {
        // The fields are internal, not private, as a map's subclass may name a field of its own the same:
        // CSharp.lua gives a subclass's field a name of its own only when it sees the base's field of that name,
        // and it does not see a private one, so in Lua the two would be one field.

        /// <summary>The package's entry for the gizmo: its place and its cell. Null before Create.</summary>
        internal Entry? entry;

        /// <summary>The gizmo's place in the cycle; made when it first has a tick.</summary>
        internal GizmoDue? due;

        /// <summary>The height, kept for the missiles of a later part.</summary>
        internal float z;

        /// <summary>The player the gizmo plays for.</summary>
        internal Player player = null!;

        /// <summary>The seconds between ticks; 0 for none.</summary>
        internal float tick;

        /// <summary>The components the class starts with, added in order when the gizmo is created: after its owner is set and before OnCreated runs. A class lists them in its constructor. Assigning a new list drops what a parent class listed.</summary>
        protected List<GizmoComponent> Components { get; set; } = new List<GizmoComponent>();

        /// <summary>The gizmo's left-to-right place. Setting it moves the gizmo's entry to the cell of the new place at once.</summary>
        [LocalSafe]
        public float X
        {
            get { return entry == null ? 0.0f : entry.X; }
            set { Place(value, Y); }
        }

        /// <summary>The gizmo's bottom-to-top place. Setting it moves the gizmo's entry to the cell of the new place at once.</summary>
        [LocalSafe]
        public float Y
        {
            get { return entry == null ? 0.0f : entry.Y; }
            set { Place(X, value); }
        }

        /// <summary>The gizmo's height. It is only kept: nothing reads it yet.</summary>
        [LocalSafe]
        public float Z
        {
            get { return z; }
            set { z = value; }
        }

        /// <summary>The gizmo's place as a vector; a vector is a table, so code that runs every tick on many gizmos reads X and Y. Setting it moves the gizmo.</summary>
        [LocalSafe]
        public Vector2 Position
        {
            get { return new Vector2(X, Y); }
            set { Place(value.X, value.Y); }
        }

        /// <summary>The player the gizmo plays for: Neutral Passive unless it was given one. The gizmo's Owner stays the owner it lives and ends with.</summary>
        [LocalSafe]
        public Player Player
        {
            get { return player; }
            set { player = value; }
        }

        /// <summary>The seconds between OnTick, rounded up to whole steps of the cycle and at least one step; 0 for no tick. A map sets it in its constructor or in OnCreated; a change takes effect after the tick that is waiting.</summary>
        [LocalSafe]
        public float Tick
        {
            get { return tick; }
            protected set
            {
                Dev.Check(value >= 0.0f, Names.NameOf(this) + ".Tick: seconds must be 0 or more");
                tick = value;
                if (entry != null)
                {
                    Arm();
                }
            }
        }

        /// <summary>Places a gizmo of the map at a place for a player, and returns it. It belongs to the owner, or else the current owner (a missile: the game), and ends with it; the player is Neutral Passive unless given. OnCreated runs with the gizmo as the current owner.</summary>
        public static T Create<T>(T gizmo, float x, float y, Player? player = null, Owner? owner = null)
            where T : Gizmo
        {
            Dev.Check(gizmo.entry == null, Names.NameOf(gizmo) + ".Create: the gizmo has been created already");
            Interactions.EnsureInstalled();
            gizmo.player = player ?? Player.NeutralPassive;
            gizmo.entry = Tracking.Begin(gizmo, Tracking.GizmoKind, x, y);
            (owner ?? gizmo.DefaultOwner()).Adopt(gizmo);
            if (!gizmo.HasEnded)
            {
                gizmo.Placed();
            }

            if (!gizmo.HasEnded)
            {
                var starting = gizmo.Components;
                for (var i = 0; i < starting.Count; i++)
                {
                    gizmo.Add(starting[i]);
                }
            }

            if (!gizmo.HasEnded)
            {
                gizmo.CallCreated();
                gizmo.Arm();
            }

            return gizmo;
        }

        /// <summary>Starts an interaction held by the gizmo, and returns it. It belongs to the owner, or else the current owner, and ends with it, with the gizmo, or by End. Fails in a development build when the gizmo has not been created, when it is not of the interaction's holder class, when its Range is not more than 0, or when it was started before.</summary>
        public T Interact<T>(T interaction, Owner? owner = null)
            where T : Interaction
        {
            Dev.Check(entry != null, Names.NameOf(this) + ".Interact: the gizmo has not been created");
            interaction.Begin(this, Tracking.GizmoKind, owner);
            return interaction;
        }

        /// <summary>Adds a component to the gizmo, and returns it. It ends with the gizmo, or by Remove. Fails in a development build when the component is on an actor already or has ended.</summary>
        public T Add<T>(T component)
            where T : GizmoComponent => AddComponent(component);

        /// <summary>Ends the gizmo: OnLeave runs and it leaves the grid. The same as End.</summary>
        public void Remove()
        {
            End();
        }

        /// <summary>The owner Create adopts the gizmo into when it is given none: the current owner. A package's kind of gizmo may choose another, as a missile belongs to the game.</summary>
        internal virtual Owner DefaultOwner()
        {
            return Owner.Current;
        }

        /// <summary>Runs in Create once the gizmo has its entry and its owner, before its components are added and OnCreated runs; nothing for a plain gizmo. A package's kind of gizmo starts its own part here, as a missile joins the physics pass.</summary>
        internal virtual void Placed()
        {
        }

        /// <summary>Runs on its own at every Tick, in the cycle's first pass, before the interactions check what is near.</summary>
        public virtual void OnTick()
        {
        }

        /// <summary>Runs once, when the gizmo has been placed, with the gizmo as the current owner.</summary>
        protected virtual void OnCreated()
        {
        }

        /// <summary>Runs when the gizmo ends, with the gizmo as the current owner. It has no place here: a Find from it does not see it, the interactions it held have ended and the pairs it was the other of have parted. It leaves its cell after; a unit, item or destructable has left the grid before its OnLeave. HasEnded is still false here.</summary>
        protected virtual void OnLeave()
        {
        }

        /// <summary>Tells the package the gizmo is ending, which ends the interactions it holds and parts the pairs it is the other of, runs OnLeave, and takes the gizmo out of the grid. A gizmo is the package's own, so core's hook for a widget ending does not see it.</summary>
        protected sealed override void OnEnd()
        {
            if (entry != null)
            {
                Tracking.Ending(entry);
            }

            if (due != null)
            {
                due.Cancel();
            }

            var previous = Owner.current;
            Owner.current = this;
            try
            {
                OnLeave();
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(this) + ".OnLeave", error);
            }

            Owner.current = previous;
            if (entry != null)
            {
                Tracking.Release(entry);
            }
        }

        /// <summary>Runs OnTick with the gizmo as the current owner, and reports an error it throws. The cycle calls it, since a map overrides the hook.</summary>
        internal void CallTick()
        {
            var previous = Owner.current;
            Owner.current = this;
            var failure = Guard.Tick(this);
            if (failure != null)
            {
                Errors.Report(Names.NameOf(this) + ".OnTick", failure);
            }

            Owner.current = previous;
        }

        /// <summary>Runs OnCreated with the gizmo as the current owner, and reports an error it throws.</summary>
        private void CallCreated()
        {
            var previous = Owner.current;
            Owner.current = this;
            try
            {
                OnCreated();
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(this) + ".OnCreated", error);
            }

            Owner.current = previous;
        }

        /// <summary>Sets the gizmo's place, and its cell with it.</summary>
        private void Place(float x, float y)
        {
            // The check's message is built only when it fails: a moving gizmo sets X and Y every tick.
            if (entry == null)
            {
                Dev.Check(false, Names.NameOf(this) + ": the gizmo has not been created");
            }

            Interactions.Grid.Move(entry!, x, y);
        }

        /// <summary>Puts the gizmo in the cycle for its next tick when it has a tick and none is waiting.</summary>
        private void Arm()
        {
            if (tick <= 0.0f || ending || HasEnded)
            {
                return;
            }

            if (due == null)
            {
                due = new GizmoDue(this);
            }

            due.Arm();
        }
    }
}
