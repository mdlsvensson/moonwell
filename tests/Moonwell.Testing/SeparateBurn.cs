namespace Moonwell.Testing
{
    /// <summary>A burn that stacks: each apply is its own buff.</summary>
    public class SeparateBurn : Burn
    {
        /// <summary>Makes a burn that stacks separately.</summary>
        public SeparateBurn()
        {
            Stacking = Stacking.Separate;
        }
    }
}
