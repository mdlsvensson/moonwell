namespace Moonwell
{
    /// <summary>
    /// An attack type, as World Editor offers it: a unit's attack in its object data, and the damage package's hits. Its
    /// numbers are World Editor's list, not the natives' (JASS's ATTACK_TYPE_MAGIC is 4, AttackType.Magic is 6): never cast
    /// one to or from a native's value.
    /// </summary>
    public enum AttackType
    {
        /// <summary>None: World Editor's "unknown".</summary>
        Unknown = 0,

        /// <summary>Normal.</summary>
        Normal = 1,

        /// <summary>Pierce.</summary>
        Pierce = 2,

        /// <summary>Siege.</summary>
        Siege = 3,

        /// <summary>Spells.</summary>
        Spells = 4,

        /// <summary>Chaos.</summary>
        Chaos = 5,

        /// <summary>Magic.</summary>
        Magic = 6,

        /// <summary>Hero.</summary>
        Hero = 7,
    }
}
