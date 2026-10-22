#if !MOONWELL_LUA

using System;

namespace Moonwell.Objects
{
    /// <summary>
    /// A hero's life and mana as its MaxLife, MaxMana and Mana read them while it has no handle: the totals it starts with,
    /// as the game gives them (seen 10-21). Most life is its base field (<c>uhpm</c>) plus its strength times the life a
    /// point of strength adds; most mana its base field (<c>umpm</c>) plus its intelligence times the mana a point adds; and
    /// starting mana its field (<c>umpi</c>) plus the same, up to its most mana. The bonuses are
    /// <see cref="StandardValues.HeroBonus"/>'s, 0 with none. A total the hero's class put (<see cref="Declared.Totals"/>)
    /// is read as it was put, starting mana still up to the most.
    /// </summary>
    /// @CSharpLua.Ignore
    internal static class HeroTotals
    {
        /// <summary>Whether a hero's shared member reads the field as a total: most life, most mana or starting mana.</summary>
        internal static bool Holds(string field)
        {
            return field == "uhpm" || field == "umpm" || field == "umpi";
        }

        /// <summary>The declared value of a hero's field: a total for most life, most mana and starting mana, else the value of the field.</summary>
        internal static T Read<T>(Declared fields, object owner, string field)
        {
            return Holds(field) ? (T)(object)Total(fields, owner, field) : fields.Get<T>(owner, field);
        }

        private static int Total(Declared fields, object owner, string field)
        {
            var total = fields.Totals.TryGetValue(field, out var put) ? (int)put! : FromAttributes(fields, owner, field);

            // Starting mana is never above the most, as the game starts the hero, put or not.
            return field == "umpi" ? Math.Min(total, Total(fields, owner, "umpm")) : total;
        }

        private static int FromAttributes(Declared fields, object owner, string field)
        {
            var bonus = StandardValues.HeroBonus?.Invoke() ?? (0.0f, 0.0f);
            switch (field)
            {
                case "uhpm":
                    return Add(fields.Get<int>(owner, "uhpm"), fields.Get<int>(owner, "ustr"), bonus.LifePerStrength);
                case "umpm":
                    return Add(fields.Get<int>(owner, "umpm"), fields.Get<int>(owner, "uint"), bonus.ManaPerIntelligence);
                default:
                    return Add(fields.Get<int>(owner, "umpi"), fields.Get<int>(owner, "uint"), bonus.ManaPerIntelligence);
            }
        }

        // The game keeps whole numbers of life and mana.
        private static int Add(int field, int attribute, float perPoint)
        {
            return (int)(field + (attribute * perPoint));
        }
    }
}

#endif
