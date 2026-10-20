namespace Moonwell
{
    /// <summary>Reactions to the damage a unit deals, on the unit's class or its components; write only the ones needed, as public methods.</summary>
    public interface IDamageSource
    {
        // Before armor: the amount, the types and the target's armor for this hit may change.
        /// @CSharpLua.Ignore
        void OnDealing(Hit hit)
        {
        }

        // After armor: the amount may change.
        /// @CSharpLua.Ignore
        void OnDealt(Hit hit)
        {
        }

        // After armor, when the hit would kill: the amount may be lowered, or Spare called.
        /// @CSharpLua.Ignore
        void OnKilling(Hit hit)
        {
        }

        // 0 s later, when the target's life has dropped.
        /// @CSharpLua.Ignore
        void OnLanded(Hit hit)
        {
        }

        // 0 s later, once for every hit the unit landed in that moment.
        /// @CSharpLua.Ignore
        void OnBurst(Burst burst)
        {
        }
    }
}
