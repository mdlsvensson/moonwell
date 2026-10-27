namespace Moonwell
{
    /// <summary>What a second apply of a buff does when the unit already has one of its class.</summary>
    public enum Stacking
    {
        /// <summary>The buff already there is refreshed: from the refresh on it lasts the new apply's duration, and it takes the new source; it keeps its other values and what it has counted, and the new buff is dropped.</summary>
        Refresh,

        /// <summary>Each apply is a buff of its own.</summary>
        Separate,

        /// <summary>One buff per source: an apply from a source that already has one refreshes that buff, as Refresh does.</summary>
        PerSource,
    }
}
