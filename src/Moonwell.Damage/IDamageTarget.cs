namespace Moonwell
{
    /// <summary>Reactions to the damage a unit takes, on the unit's class or its components (buffs too); write only the ones needed.</summary>
    public interface IDamageTarget
    {
        /// <summary>Before armor: the amount, the types and this unit's armor for this hit may change.</summary>
        void OnTaking(Hit hit)
        {
        }

        /// <summary>After armor: the amount may change.</summary>
        void OnTaken(Hit hit)
        {
        }

        /// <summary>After armor, when the hit would kill this unit: the amount may be lowered, or Spare called.</summary>
        void OnLethal(Hit hit)
        {
        }

        /// <summary>0 s later, when this unit's life has dropped.</summary>
        void OnHurt(Hit hit)
        {
        }
    }
}
