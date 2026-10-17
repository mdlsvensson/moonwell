namespace Moonwell.Testing
{
    /// <summary>A burn with one buff per source: a second apply from the same source refreshes it.</summary>
    public class SourceBurn : Burn
    {
        /// <summary>Makes a burn that stacks per source.</summary>
        public SourceBurn()
        {
            Stacking = Stacking.PerSource;
        }
    }
}
