namespace Moonwell
{
    /// <summary>
    /// A defense (armor) type, as World Editor offers it: a unit's defense in its object data, and the damage package's
    /// hits. Its numbers are World Editor's list, not the natives': never cast one to or from a native's value.
    /// </summary>
    public enum DefenseType
    {
        /// <summary>Normal.</summary>
        Normal = 0,

        /// <summary>Small.</summary>
        Small = 1,

        /// <summary>Medium.</summary>
        Medium = 2,

        /// <summary>Large.</summary>
        Large = 3,

        /// <summary>Fortified.</summary>
        Fort = 4,

        /// <summary>Hero.</summary>
        Hero = 5,

        /// <summary>Divine.</summary>
        Divine = 6,

        /// <summary>Unarmored: World Editor's "none".</summary>
        None = 7,
    }
}
