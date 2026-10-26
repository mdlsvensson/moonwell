namespace Moonwell
{
    /// <summary>Reactions to the damage a unit takes, on the unit's class or its components (buffs too); write only the ones needed, as public methods.</summary>
    public interface IDamageTarget
    {
        /// <summary>Before armor: the hit may change its amount, its types and this unit's armor for this hit.</summary>
        /// @CSharpLua.Ignore
        void OnTaking(Hit hit)
        {
        }

        /// <summary>After armor: the hit may change its amount.</summary>
        /// @CSharpLua.Ignore
        void OnTaken(Hit hit)
        {
        }

        /// <summary>After armor, when the hit would kill this unit: the amount may be lowered, or <see cref="Hit.Spare"/> called.</summary>
        /// @CSharpLua.Ignore
        void OnDying(Hit hit)
        {
        }

        /// <summary>0 s later, when this unit's life has dropped; <see cref="Hit.Killed"/> tells whether this hit killed it.</summary>
        /// @CSharpLua.Ignore
        void OnHurt(Hit hit)
        {
        }
    }
}
