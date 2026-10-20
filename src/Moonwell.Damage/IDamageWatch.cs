namespace Moonwell
{
    /// <summary>Map-wide reactions to damage, on the game's components (every hit) and a player's (hits where the player owns the source or the target); write only the ones needed.</summary>
    public interface IDamageWatch
    {
        /// <summary>Before armor: the amount, the types and the target's armor for this hit may change.</summary>
        void OnBeforeArmor(Hit hit)
        {
        }

        /// <summary>After armor: the amount may change.</summary>
        void OnAfterArmor(Hit hit)
        {
        }

        /// <summary>After armor, when the hit would kill: the amount may be lowered, or Spare called.</summary>
        void OnLethal(Hit hit)
        {
        }

        /// <summary>0 s later, when the target's life has dropped.</summary>
        void OnLanded(Hit hit)
        {
        }

        /// <summary>0 s later, once for every source that landed hits in that moment.</summary>
        void OnBurst(Burst burst)
        {
        }
    }
}
