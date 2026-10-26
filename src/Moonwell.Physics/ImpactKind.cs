namespace Moonwell
{
    /// <summary>What a missile met at an impact.</summary>
    public enum ImpactKind
    {
        /// <summary>A unit.</summary>
        Unit,

        /// <summary>A destructable.</summary>
        Destructable,

        /// <summary>An item.</summary>
        Item,

        /// <summary>Another missile.</summary>
        Missile,

        /// <summary>A wall.</summary>
        Wall,

        /// <summary>The ground: the terrain, water's surface, or a bridge's deck.</summary>
        Ground,
    }
}
