namespace Moonwell
{
    /// <summary>What a second apply of a buff does when the unit already has one of its class.</summary>
    public enum Stacking
    {
        /// <summary>The buff already there gets the new duration and source.</summary>
        Refresh,

        /// <summary>Each apply is a buff of its own.</summary>
        Separate,

        /// <summary>One buff per source: an apply from a source that already has one refreshes that buff.</summary>
        PerSource,
    }
}
