using Moonwell.Hosting;

namespace Moonwell
{
    /// <summary>Registers each game event with the world once, when a reaction first needs it.</summary>
    internal static class Events
    {
        /// <summary>Whether the world already listens for each game event, indexed by GameEvent.</summary>
        private static bool[] listening = new bool[5];

        /// <summary>The reaction bits that some class overrides, once any has been needed.</summary>
        private static int wanted;

        /// <summary>Adds the reaction bits to the wanted ones, and listens for the event each bit needs.</summary>
        internal static void Need(int mask)
        {
            wanted |= mask;
            if ((mask & (Hook.OnDamaged | Hook.OnDealtDamage | Hook.OnUnitDamaged)) != 0)
            {
                Listen(GameEvent.Damage);
            }

            if ((mask & (Hook.OnDeath | Hook.OnKill | Hook.OnUnitDied)) != 0)
            {
                Listen(GameEvent.Death);
            }

            if ((mask & (Hook.OnCast | Hook.OnUnitCast)) != 0)
            {
                Listen(GameEvent.Spell);
            }

            if ((mask & (Hook.OnItemPickedUp | Hook.OnPickedUp)) != 0)
            {
                Listen(GameEvent.PickUp);
            }

            if ((mask & (Hook.OnItemDropped | Hook.OnDropped)) != 0)
            {
                Listen(GameEvent.Drop);
            }
        }

        /// <summary>Whether a reaction bit has been needed.</summary>
        internal static bool Wants(int bit)
        {
            return (wanted & bit) != 0;
        }

        /// <summary>Forgets every registration and need; Runtime.Reset calls it.</summary>
        internal static void Reset()
        {
            listening = new bool[5];
            wanted = 0;
        }

        /// <summary>Asks the world to deliver an event, once.</summary>
        private static void Listen(GameEvent gameEvent)
        {
            var index = (int)gameEvent;
            if (listening[index])
            {
                return;
            }

            listening[index] = true;
            Runtime.World.Listen(gameEvent);
        }
    }
}
