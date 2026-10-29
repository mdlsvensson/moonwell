namespace Moonwell
{
    /// <summary>The reaction of a unit a missile strikes, on the unit's class, its components or its buffs; write it as a public method where it is needed. It runs at each contact, after the missile's OnImpact and its components', before the damage and the outcome, so the defender has the last word (a shield that sends missiles back sets the impact's Contact to Bounce and its Damage to 0). It runs on the class first, then on the components and buffs in the order they were added, each guarded.</summary>
    public interface IMissileTarget
    {
        /// <summary>A missile struck the unit. The impact may be changed: its Contact (the missile's outcome) and its Damage.</summary>
        /// <param name="impact">The contact: the point, the normal out of the unit, the closing speed, the outcome and the damage.</param>
        /// @CSharpLua.Ignore
        void OnStruck(Impact impact)
        {
        }
    }
}
