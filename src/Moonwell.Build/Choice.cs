using System.Globalization;
using System.Reflection;

namespace Moonwell.Build
{
    /// <summary>
    /// A choice enum of <c>Moonwell.Objects</c> (<c>ArmorType</c>, <c>Targets</c>), or of core Moonwell (<c>AttackType</c>, whose
    /// texts <c>Moonwell.Objects</c> keeps in <c>CoreChoices</c>), as the build reads and writes it: each member's number and the
    /// game's texts from its <c>[GameValue]</c>. The build turns the standard data's texts into the numbers the map's Lua reads,
    /// and a declared number into the text or number the object file holds.
    /// </summary>
    public sealed class Choice
    {
        private const string GameValueName = "Moonwell.Objects.GameValueAttribute";

        /// <param name="name">The enum's name.</param>
        /// <param name="flags">Whether it is a <c>[Flags]</c> enum, whose field holds any of its members.</param>
        /// <param name="members">The members, in the enum's order.</param>
        public Choice(string name, bool flags, IReadOnlyList<Member> members)
        {
            Name = name;
            Flags = flags;
            Members = members;
        }

        /// <summary>The enum's name.</summary>
        public string Name { get; }

        /// <summary>Whether the field holds any of the members: their bits, written as a comma list of their texts or as the number.</summary>
        public bool Flags { get; }

        /// <summary>The members, in the enum's order.</summary>
        public IReadOnlyList<Member> Members { get; }

        /// <summary>One member of the enum.</summary>
        /// <param name="Number">The member's number.</param>
        /// <param name="Text">How the object file spells it.</param>
        /// <param name="Alternates">The other spellings the game's data uses for it.</param>
        public sealed record Member(int Number, string Text, IReadOnlyList<string> Alternates);

        /// <summary>
        /// The number of the game's text, as <c>Moonwell.Objects</c> reads it on .NET (<c>StandardValues.Parse</c>): a member's text or
        /// other spelling without regard to case; for flags a whole number as it is, else a comma list of members' texts, leaving out
        /// a part that is none; else a whole number, a decimal truncated, else 0. Null gives 0.
        /// </summary>
        /// <param name="text">The text.</param>
        /// <returns>The number.</returns>
        public int Number(string? text)
        {
            if (text == null)
            {
                return 0;
            }

            var trimmed = text.Trim();
            if (Flags)
            {
                if (int.TryParse(trimmed, NumberStyles.Integer, CultureInfo.InvariantCulture, out var bits))
                {
                    return bits;
                }

                var sum = 0;
                foreach (var part in trimmed.Split(','))
                {
                    sum |= Find(part.Trim())?.Number ?? 0;
                }

                return sum;
            }

            var member = Find(trimmed);
            if (member != null)
            {
                return member.Number;
            }

            if (int.TryParse(trimmed, NumberStyles.Integer, CultureInfo.InvariantCulture, out var whole))
            {
                return whole;
            }

            return float.TryParse(trimmed, NumberStyles.Float, CultureInfo.InvariantCulture, out var truncated) ? (int)truncated : 0;
        }

        /// <summary>The number of a text that names members only (for flags, a comma list of them) or is a whole number; else null.</summary>
        /// <param name="text">The text.</param>
        /// <returns>The number, or null.</returns>
        public int? ExactNumber(string text)
        {
            var trimmed = text.Trim();
            if (int.TryParse(trimmed, NumberStyles.Integer, CultureInfo.InvariantCulture, out var whole))
            {
                return whole;
            }

            if (!Flags)
            {
                return Find(trimmed)?.Number;
            }

            if (trimmed.Length == 0)
            {
                return 0;
            }

            var sum = 0;
            foreach (var part in trimmed.Split(','))
            {
                var member = Find(part.Trim());
                if (member == null)
                {
                    return null;
                }

                sum |= member.Number;
            }

            return sum;
        }

        /// <summary>The text the object file holds for a number: the member's, or for flags the comma list of the members whose bits it has, in the enum's order.</summary>
        /// <param name="owner">The declared class, for messages.</param>
        /// <param name="field">The field id, for messages.</param>
        /// <param name="number">The number.</param>
        /// <returns>The text.</returns>
        /// <exception cref="BuildException">No member has the number, or for flags a bit of it is no member's.</exception>
        public string Text(string owner, string field, int number)
        {
            if (!Flags)
            {
                foreach (var member in Members)
                {
                    if (member.Number == number)
                    {
                        return member.Text;
                    }
                }

                throw new BuildException($"{owner}: {field} is {number.ToString(CultureInfo.InvariantCulture)}, which is no value of {Name}");
            }

            var texts = new List<string>();
            var left = number;
            foreach (var member in Members)
            {
                if (member.Number != 0 && (number & member.Number) == member.Number)
                {
                    texts.Add(member.Text);
                    left &= ~member.Number;
                }
            }

            if (left != 0)
            {
                throw new BuildException(
                    $"{owner}: {field} is {number.ToString(CultureInfo.InvariantCulture)}, whose bit {left.ToString(CultureInfo.InvariantCulture)} is no value of {Name}");
            }

            return string.Join(",", texts);
        }

        /// <summary>The choice of an enum of the map's <c>Moonwell.Objects</c>, read through its metadata.</summary>
        /// <param name="type">The enum, loaded in the build's load context.</param>
        /// <returns>The choice.</returns>
        internal static Choice Read(Type type)
        {
            var members = new List<Member>();
            foreach (var field in type.GetFields(BindingFlags.Public | BindingFlags.Static))
            {
                var value = field.GetCustomAttributesData().FirstOrDefault(attribute => attribute.AttributeType.FullName == GameValueName);
                if (value == null)
                {
                    continue;
                }

                var text = (string)value.ConstructorArguments[0].Value!;
                var alternates = value.ConstructorArguments.Count > 1 && value.ConstructorArguments[1].Value is IReadOnlyCollection<CustomAttributeTypedArgument> others
                    ? others.Select(other => (string)other.Value!).ToList()
                    : new List<string>();
                members.Add(new Member(Convert.ToInt32(field.GetRawConstantValue(), CultureInfo.InvariantCulture), text, alternates));
            }

            return new Choice(type.Name, type.IsDefined(typeof(FlagsAttribute), false), members);
        }

        private Member? Find(string text)
        {
            if (text.Length == 0)
            {
                return null;
            }

            foreach (var member in Members)
            {
                if (string.Equals(member.Text, text, StringComparison.OrdinalIgnoreCase)
                    || member.Alternates.Any(other => string.Equals(other, text, StringComparison.OrdinalIgnoreCase)))
                {
                    return member;
                }
            }

            return null;
        }
    }
}
