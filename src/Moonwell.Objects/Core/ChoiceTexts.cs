#if !MOONWELL_LUA

using System;
using System.Collections.Generic;
using System.Globalization;
using System.Reflection;

namespace Moonwell.Objects
{
    /// <summary>Reads the game's text of a choice into its enum's number, on .NET: the build's Lua side reads it the same way.</summary>
    /// @CSharpLua.Ignore
    internal static class ChoiceTexts
    {
        // A Dictionary under a lock, as in FieldChecks: the Lua runtime has no System.Collections.Concurrent.
        private static readonly Dictionary<Type, List<KeyValuePair<int, string[]>>> membersOf = new Dictionary<Type, List<KeyValuePair<int, string[]>>>();

        /// <summary>
        /// The number of a choice's text. A member whose game text or other spelling is the text, without regard to case, gives its
        /// number; a flags enum reads a whole number as it is (a field stored as a number) and a comma list as its members' bits,
        /// leaving out a part that is no member's; another enum reads a text that is no member's as a whole number, a decimal
        /// truncated, else 0. Null gives 0.
        /// </summary>
        internal static int Number(Type type, string? text)
        {
            if (text == null)
            {
                return 0;
            }

            var members = MembersOf(type);
            var trimmed = text.Trim();
            if (type.IsDefined(typeof(FlagsAttribute), false))
            {
                if (int.TryParse(trimmed, NumberStyles.Integer, CultureInfo.InvariantCulture, out var bits))
                {
                    return bits;
                }

                var sum = 0;
                foreach (var part in trimmed.Split(','))
                {
                    sum |= Find(members, part.Trim()) ?? 0;
                }

                return sum;
            }

            var member = Find(members, trimmed);
            if (member.HasValue)
            {
                return member.Value;
            }

            if (int.TryParse(trimmed, NumberStyles.Integer, CultureInfo.InvariantCulture, out var whole))
            {
                return whole;
            }

            return float.TryParse(trimmed, NumberStyles.Float, CultureInfo.InvariantCulture, out var truncated) ? (int)truncated : 0;
        }

        private static int? Find(List<KeyValuePair<int, string[]>> members, string text)
        {
            if (text.Length == 0)
            {
                return null;
            }

            foreach (var member in members)
            {
                foreach (var spelling in member.Value)
                {
                    if (string.Equals(spelling, text, StringComparison.OrdinalIgnoreCase))
                    {
                        return member.Key;
                    }
                }
            }

            return null;
        }

        private static List<KeyValuePair<int, string[]>> MembersOf(Type type)
        {
            lock (membersOf)
            {
                if (!membersOf.TryGetValue(type, out var members))
                {
                    members = new List<KeyValuePair<int, string[]>>();
                    foreach (var field in TextsOf(type).GetFields(BindingFlags.Public | BindingFlags.Static))
                    {
                        var value = field.GetCustomAttribute<GameValueAttribute>(false);
                        if (value != null)
                        {
                            var spellings = new string[value.Alternates.Length + 1];
                            spellings[0] = value.Text;
                            value.Alternates.CopyTo(spellings, 1);
                            members.Add(new KeyValuePair<int, string[]>(Convert.ToInt32(field.GetValue(null), CultureInfo.InvariantCulture), spellings));
                        }
                    }

                    membersOf[type] = members;
                }

                return members;
            }
        }

        // The enum whose members carry the game's texts: a core enum's (Moonwell.AttackType) are in the generated CoreChoices,
        // under its name, found as the build finds them.
        private static Type TextsOf(Type type)
        {
            return type.Namespace == "Moonwell" ? typeof(ChoiceTexts).Assembly.GetType("Moonwell.Objects.CoreChoices+" + type.Name) ?? type : type;
        }
    }
}

#endif
