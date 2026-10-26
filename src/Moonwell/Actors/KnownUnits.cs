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

        /// <summary>The object of a game unit when the runtime holds it; null otherwise.</summary>
        internal static Unit? Find(unit handle)
        {
            if (known.TryGetValue(handle, out var found))
            {
                return found;
            }

            return null;
        }

        /// <summary>Takes a game unit in: makes its object, adds its starting components, and runs OnCreated and the map's OnUnitCreated.</summary>
        internal static Unit? TakeIn(unit handle, Owner? owner)
        {
            var existing = Find(handle);
            if (existing != null)
            {
                return existing;
            }

            var typeId = Runtime.World.UnitTypeId(handle);
            if (typeId == 0)
            {
                return null;
            }

            var maker = ObjectTypes.UnitMaker(typeId);
            var made = maker != null ? maker() : Plain(Runtime.World.IsHero(handle));
            made.Handle = handle;
            made.TypeId = typeId;
            made.owner = Runtime.World.OwnerOf(handle);
            known[handle] = made;
            Live.Add(made);
            owner?.Adopt(made);
            made.Mask = Hooks.MaskOf(made, Unit.UnitHooks);
            Events.Need(made.Mask);
            for (var i = 0; i < made.StartComponents.Count; i++)
            {
                made.Add(made.StartComponents[i]);
            }

            if (!made.HasEnded)
            {
                var previous = Owner.current;
                Owner.current = made;
                try
                {
                    made.CallEnter();
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(made) + ".OnCreated", error);
                }

                Owner.current = previous;
            }

            if (Events.Wants(Hook.OnUnitCreated) && !made.HasEnded)
            {
                EnterComponents(Game.Instance, made);
                var player = Player.Existing(made.Player.Slot);
                if (player != null)
                {
                    EnterComponents(player, made);
                }
            }

            if (!made.HasEnded)
            {
                Hooks.CallTakenIn(made);
            }

            return made;
        }

        /// <summary>The object of a unit whose type no class is registered for: a Hero for a hero, a Unit for any other.</summary>
        internal static Unit Plain(bool hero)
        {
            return hero ? new Hero() : new Unit();
        }

        /// <summary>Forgets a unit the game has removed, so that its handle makes no object again.</summary>
        internal static void Forget(Unit unit)
        {
            known.Remove(unit.Handle);
            Live.Remove(unit);
        }

        /// <summary>Forgets every unit; Runtime.Reset calls it.</summary>
        internal static void Reset()
        {
            known.Clear();
            Live.Clear();
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

                var previous = Owner.current;
                Owner.current = component;
                try
                {
                    component.OnUnitCreated(unit);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(component) + ".OnUnitCreated", error);
                }

                Owner.current = previous;
            }

            actor.EndDispatch();
        }
    }
}
