namespace Moonwell.Build
{
    /// <summary>What a hero's attributes add to its life and mana, from the gameplay constants.</summary>
    /// <param name="LifePerStrength">The life each point of strength adds (<c>StrHitPointBonus</c>).</param>
    /// <param name="ManaPerIntelligence">The mana each point of intelligence adds (<c>IntManaBonus</c>).</param>
    public sealed record HeroBonus(float LifePerStrength, float ManaPerIntelligence)
    {
        /// <summary>
        /// The attribute whose points add to a hero's field: strength to most life (<c>uhpm</c>), intelligence to most mana
        /// (<c>umpm</c>) and starting mana (<c>umpi</c>); null for any other field. A hero's MaxLife, MaxMana and Mana are these
        /// fields' totals, as the game starts the hero (seen 10-21).
        /// </summary>
        internal static string? AttributeOf(string field)
        {
            switch (field)
            {
                case "uhpm":
                    return "ustr";
                case "umpm":
                case "umpi":
                    return "uint";
                default:
                    return null;
            }
        }

        /// <summary>What a point of the attribute of a field (<see cref="AttributeOf"/>) adds to it.</summary>
        internal float PerPoint(string field)
        {
            return field == "uhpm" ? LifePerStrength : ManaPerIntelligence;
        }

        /// <summary>A total as the game keeps it, a whole number: the field plus the attribute's points times what each adds (HeroTotals in Moonwell.Objects).</summary>
        internal static int Total(int field, int attribute, float perPoint)
        {
            return (int)(field + (attribute * perPoint));
        }
    }
}
