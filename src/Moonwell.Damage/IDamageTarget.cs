namespace Moonwell
{
    /// <summary>Reactions to the damage a unit takes, on the unit's class or its components (buffs too); write only the ones needed, as public methods.</summary>
    public interface IDamageTarget
    {
        // Before armor: the amount, the types and this unit's armor for this hit may change.
        /// @CSharpLua.Ignore
        void OnTaking(Hit hit)
        {
        }

        // After armor: the amount may change.
        /// @CSharpLua.Ignore
        void OnTaken(Hit hit)
        {
        }

        // After armor, when the hit would kill this unit: the amount may be lowered, or Spare called.
        /// @CSharpLua.Ignore
        void OnLethal(Hit hit)
        {
        }

        // 0 s later, when this unit's life has dropped.
        /// @CSharpLua.Ignore
        void OnHurt(Hit hit)
        {
        }
    }
}
