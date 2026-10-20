namespace Moonwell
{
    /// <summary>Map-wide reactions to damage, on the game's components (every hit) and a player's (hits where the player owns the source or the target); write only the ones needed, as public methods.</summary>
    public interface IDamageWatch
    {
        // Before armor: the amount, the types and the target's armor for this hit may change.
        /// @CSharpLua.Ignore
        void OnBeforeArmor(Hit hit)
        {
        }

        // After armor: the amount may change.
        /// @CSharpLua.Ignore
        void OnAfterArmor(Hit hit)
        {
        }

        // After armor, when the hit would kill: the amount may be lowered, or Spare called.
        /// @CSharpLua.Ignore
        void OnLethal(Hit hit)
        {
        }

        // 0 s later, when the target's life has dropped.
        /// @CSharpLua.Ignore
        void OnLanded(Hit hit)
        {
        }

        // 0 s later, once for every source that landed hits in that moment.
        /// @CSharpLua.Ignore
        void OnBurst(Burst burst)
        {
        }
    }
}
