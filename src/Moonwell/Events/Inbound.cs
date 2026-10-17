using System;

namespace Moonwell
{
    /// <summary>The events the world delivers to the runtime, and what the runtime does with each.</summary>
    internal static class Inbound
    {
        /// <summary>Runs OnGameStart on the game's components, then on each existing player's, in slot order.</summary>
        internal static void GameStarted()
        {
            StartComponents(Game.Instance);
            for (var slot = 0; slot < Player.MaxSlots; slot++)
            {
                var existing = Player.Existing(slot);
                if (existing != null)
                {
                    StartComponents(existing);
                }
            }
        }

        /// <summary>A unit enters the map: it is taken in when its type is registered, or when a component wants to hear of entries.</summary>
        internal static void Entered(unit handle)
        {
            if (Units.Find(handle) != null)
            {
                return;
            }

            if (ObjectTypes.UnitMaker(Runtime.World.UnitTypeId(handle)) != null || Events.Wants(Hook.OnUnitEnter))
            {
                Units.TakeIn(handle, null);
            }
        }

        /// <summary>Runs OnGameStart on the map components of an actor, in add order.</summary>
        private static void StartComponents(Actor actor)
        {
            var count = actor.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = actor.ComponentAt(i) as MapComponent;
                if (component == null || component.HasEnded || (component.Mask & Hook.OnGameStart) == 0)
                {
                    continue;
                }

                var previous = Owner.current;
                Owner.current = component;
                try
                {
                    component.OnGameStart();
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(component) + ".OnGameStart", error);
                }

                Owner.current = previous;
            }

            actor.EndDispatch();
        }
    }
}
