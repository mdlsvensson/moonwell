using System;
using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>The units the runtime holds: the object of each game unit, found by its handle, and the live ones in order.</summary>
    internal static class KnownUnits
    {
        /// <summary>The object of each unit the runtime holds, keyed by the game's handle; a lookup only.</summary>
        private static Dictionary<unit, Unit> known = new Dictionary<unit, Unit>();

        /// <summary>The units the runtime holds, in the order they were taken in.</summary>
        internal static OrderedSet<Unit> Live = new OrderedSet<Unit>();

        /// <summary>The units the runtime removed whose type id still read when they ended, as the game's does in the frame of RemoveUnit, keyed by the handle, in the order removed: each handle stays mapped to its ended object, which TakeIn does not take in again, so an event the game fires after the removal (an item's drop, the enter of a unit made in the same instant) makes no second object. The sweep forgets each once its type id reads 0.</summary>
        private static OrderedMap<unit, Unit> removed = new OrderedMap<unit, Unit>();

        /// <summary>Whether every unit that enters the map is taken in, as the interactions package asks once units are tracked; otherwise only a unit of a registered type, or every unit while something wants OnUnitCreated.</summary>
        internal static bool EveryUnit;

        /// <summary>The object of a game unit when the runtime holds it; null otherwise.</summary>
        internal static Unit? Find(unit handle)
        {
            if (known.TryGetValue(handle, out var found))
            {
                return found;
            }

            return null;
        }

        /// <summary>Takes a game unit in: makes its object, adds its starting components, and runs OnCreated, and the map's OnUnitCreated when the unit is coming into the game (entering: it enters, Unit.Create made it, or it is there at the start), not for a unit taken in long after it came in. An illusion is taken in as a plain unit, not as its type's class, with no starting components and no OnCreated (decision 77).</summary>
        internal static Unit? TakeIn(unit handle, Scope? scope, bool entering)
        {
            var existing = Find(handle);
            if (existing != null)
            {
                return existing;
            }

            // A unit the runtime removed is not taken in again, though its type id still reads in that frame.
            if (removed.ContainsKey(handle))
            {
                return null;
            }

            var typeId = Runtime.World.UnitTypeId(handle);
            if (typeId == 0)
            {
                return null;
            }

            var illusion = Runtime.World.IsIllusion(handle);
            var maker = illusion ? null : ObjectTypes.UnitMaker(typeId);
            var made = maker != null ? maker() : Plain(Runtime.World.IsHero(handle));
            made.Handle = handle;
            made.TypeId = typeId;
            made.IsIllusion = illusion;
            made.owner = Runtime.World.OwnerOf(handle);
            known[handle] = made;
            Live.Add(made);
            scope?.Adopt(made);
            made.Mask = Hooks.MaskOf(made, Unit.UnitHooks);
            Events.Need(made.Mask);
            for (var i = 0; i < made.StartComponents.Count; i++)
            {
                made.Add(made.StartComponents[i]);
            }

            // From here the unit has come in: its OnLeave and the ending listeners run when it ends. One that ended before
            // (an owner that has ended ends it at once) runs neither OnCreated nor OnLeave.
            if (!made.HasEnded)
            {
                made.entered = true;
            }

            if (!made.HasEnded && !illusion)
            {
                var previous = Scope.current;
                Scope.current = made;
                try
                {
                    made.CallEnter();
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(made) + ".OnCreated", error);
                }

                Scope.current = previous;
            }

            if (entering && Events.Wants(Hook.OnUnitCreated) && !made.HasEnded)
            {
                EnterComponents(Game.Instance, made);
                var player = Player.Existing(made.Owner.Slot);
                if (player != null)
                {
                    EnterComponents(player, made);
                }
            }

            if (!made.HasEnded)
            {
                Hooks.CallTakenIn(made);
            }
#if DEBUG
            if (!made.HasEnded)
            {
                DebugSession.UnitTakenIn(made);
            }
#endif

            return made;
        }

        /// <summary>The object of a unit whose type no class is registered for: a Hero for a hero, a Unit for any other.</summary>
        internal static Unit Plain(bool hero)
        {
            return hero ? new Hero() : new Unit();
        }

        /// <summary>Forgets a unit that is ending, as it leaves the game, so that its handle makes no object again. A handle whose type id still reads (the runtime removed the unit in this frame) is kept in removed until the sweep reads its type id 0.</summary>
        internal static void Forget(Unit unit)
        {
            Live.Remove(unit);
            if (unit.Handle == null)
            {
                return;
            }

            known.Remove(unit.Handle);
            if (Runtime.World.UnitTypeId(unit.Handle) != 0)
            {
                removed[unit.Handle] = unit;
            }
        }

        /// <summary>Forgets the handles of the units the runtime removed whose type id now reads 0; the sweep calls it.</summary>
        internal static void ForgetRemoved()
        {
            for (var i = removed.Count - 1; i >= 0; i--)
            {
                var handle = removed.KeyAt(i);
                if (Runtime.World.UnitTypeId(handle) == 0)
                {
                    removed.Remove(handle);
                }
            }
        }

        /// <summary>From now on takes in every unit that enters the map, and takes in now each unit in the world the runtime does not hold: units the game makes with no Moonwell code (trained, summoned, made by a native) are known from then on. The interactions package calls it when units are first asked for, which is synced code, so every machine takes in the same units.</summary>
        internal static void TakeInEveryUnit()
        {
            EveryUnit = true;
            var handles = Runtime.World.UnitsInWorld();
            for (var i = 0; i < handles.Count; i++)
            {
                if (Find(handles[i]) != null)
                {
                    continue;
                }

                try
                {
                    TakeIn(handles[i], null, false);
                }
                catch (Exception error)
                {
                    Errors.Report("Moonwell.TakeIn", error);
                }
            }
        }

        /// <summary>Forgets every unit; Runtime.Reset calls it.</summary>
        internal static void Reset()
        {
            known.Clear();
            Live.Clear();
            removed.Clear();
            EveryUnit = false;
        }

        /// <summary>Runs OnUnitCreated on the map components of an actor, in add order.</summary>
        private static void EnterComponents(Actor actor, Unit unit)
        {
            var count = actor.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = actor.ComponentAt(i) as MapComponent;
                if (component == null || component.HasEnded || (component.Mask & Hook.OnUnitCreated) == 0)
                {
                    continue;
                }

                var previous = Scope.current;
                Scope.current = component;
                try
                {
                    component.OnUnitCreated(unit);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(component) + ".OnUnitCreated", error);
                }

                Scope.current = previous;
            }

            actor.EndDispatch();
        }
    }
}
