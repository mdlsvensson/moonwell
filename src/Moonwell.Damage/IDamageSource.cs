namespace Moonwell
{
    /// <summary>Reactions to the damage a unit deals, on the unit's class or its components; write only the ones needed, as public methods.</summary>
    public interface IDamageSource
    {
        /// <summary>Before armor: the hit may change its amount, its types and the target's armor for this hit.</summary>
        /// @CSharpLua.Ignore
        void OnDealing(Hit hit)
        {
        }

        /// <summary>After armor: the hit may change its amount.</summary>
        /// @CSharpLua.Ignore
        void OnDealt(Hit hit)
        {
        }

        /// <summary>After armor, when the hit would kill: the amount may be lowered, or <see cref="Hit.Spare"/> called.</summary>
        /// @CSharpLua.Ignore
        void OnKilling(Hit hit)
        {
        }

        /// <summary>0 s later, when the target's life has dropped; <see cref="Hit.Killed"/> tells whether this hit killed.</summary>
        /// @CSharpLua.Ignore
        void OnLanded(Hit hit)
        {
        }

        /// <summary>0 s later, once for all the hits the unit landed in that moment.</summary>
        /// @CSharpLua.Ignore
        void OnBurst(Burst burst)
        {
        }
    }
}
