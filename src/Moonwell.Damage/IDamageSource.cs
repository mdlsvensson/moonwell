namespace Moonwell
{
    /// <summary>Reactions to the damage a unit deals, on the unit's class or its components; write only the ones needed.</summary>
    public interface IDamageSource
    {
        /// <summary>Before armor: the amount, the types and the target's armor for this hit may change.</summary>
        void OnDealing(Hit hit)
        {
        }

        /// <summary>After armor: the amount may change.</summary>
        void OnDealt(Hit hit)
        {
        }

        /// <summary>After armor, when the hit would kill: the amount may be lowered, or Spare called.</summary>
        void OnKilling(Hit hit)
        {
        }

        /// <summary>0 s later, when the target's life has dropped.</summary>
        void OnLanded(Hit hit)
        {
        }

        /// <summary>0 s later, once for every hit the unit landed in that moment.</summary>
        void OnBurst(Burst burst)
        {
        }
    }
}
