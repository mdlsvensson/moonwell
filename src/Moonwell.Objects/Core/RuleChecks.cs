using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Globalization;
using System.Reflection;

namespace Moonwell.Objects
{
    /// <summary>
    /// Checks a value a declared class puts against the rules its field's property carries (<see cref="LimitsAttribute"/>,
    /// <see cref="NonNegativeAttribute"/>, <see cref="RequiredAttribute"/>, <see cref="MaxLengthAttribute"/>,
    /// <see cref="MaxEntriesAttribute"/>) and, for a choice, against its enum; a value that breaks one throws a
    /// <see cref="FieldRuleException"/> from where it was put. A value is never changed.
    /// </summary>
    /// <remarks>
    /// <para>What each rule allows, as the message shows it after "allows":</para>
    /// <list type="bullet">
    /// <item>A range: <c>1 to 500000</c>, <c>1 or more</c> with no upper limit, <c>at most 9</c> with no lower one; then
    /// <c>; [BeyondLimits] goes past it</c>. A number list held as text adds <c> for each number</c> before that.</item>
    /// <item>A negative where none is allowed: the range without the <c>[BeyondLimits]</c> part, <c>0 or more</c> when the
    /// field has no range, and <c>0 or more, even with [BeyondLimits]</c> when the class lifts the field's limits.</item>
    /// <item>An empty or null name: <c>any text that is not empty</c>.</item>
    /// <item>A text too long: <c>at most 50 characters; [BeyondLimits] goes past it</c>.</item>
    /// <item>A list too long: <c>at most 12 entries; [BeyondLimits] goes past it</c>.</item>
    /// <item>A choice that is no value of its enum: <c>only the values of AttackType</c>, and for a flags enum
    /// <c>only the values of Targets and their combinations</c>.</item>
    /// </list>
    /// <para>
    /// The value is shown as C# would print it in the invariant culture (<c>600000</c>, <c>0.5</c>, a choice as its number);
    /// a text in double quotes, with <c> (51 characters)</c> or <c> (33 entries)</c> after it when it is too long; an
    /// object list as <c>13 entries</c>; null as <c>null</c>. The limits are printed as doubles in the invariant culture.
    /// </para>
    /// </remarks>
    /// @CSharpLua.Ignore
    internal static class RuleChecks
    {
        private const string Lift = "; [BeyondLimits] goes past it";

        // A Dictionary under a lock, as in FieldChecks: the Lua runtime has no System.Collections.Concurrent. Its values
        // are typed object: CSharp.lua writes Lua for the generic types a file uses even when its class is ignored, and a
        // generic type naming the ignored Rules would name a type the script does not have.
        /// <summary>The rules of each field of each class (a <see cref="Rules"/>, or null for no property), found the first time a value of the field is put.</summary>
        private static readonly Dictionary<Type, Dictionary<string, object?>> rulesByClass = new Dictionary<Type, Dictionary<string, object?>>();

        /// <summary>Checks a value put on a field, or on one level of it; for a formula of levels, each level is checked when its levels are counted.</summary>
        internal static void Put(object owner, string field, int? level, object? value)
        {
            var rules = RulesOf(owner.GetType(), field);
            if (rules == null)
            {
                return;
            }

            if (value is ILevelValues levels)
            {
                if (levels.Counted)
                {
                    for (var i = 0; i < levels.Count; i++)
                    {
                        Check(rules, i + 1, levels.ValueAt(i), null);
                    }
                }
                else
                {
                    // Skip this frame: the put site is the setter and the class's constructor above it.
                    var putAt = new StackTrace(1, true);
                    levels.WhenCounted((at, one) => Check(rules, at, one, putAt));
                }

                return;
            }

            Check(rules, level, value, null);
        }

        /// <summary>Checks the number of entries of an object list after it grew.</summary>
        internal static void Entries(object owner, string field, int count)
        {
            var rules = RulesOf(owner.GetType(), field);
            if (rules != null)
            {
                CheckEntries(rules, null, count, putAt: null);
            }
        }

        private static void CheckEntries(Rules rules, int? level, int count, StackTrace? putAt)
        {
            if (rules.MaxEntries.HasValue && !rules.Lifted && count > rules.MaxEntries.Value)
            {
                throw rules.Error(Count(count) + " entries", "at most " + Count(rules.MaxEntries.Value) + " entries" + Lift, level, putAt);
            }
        }

        private static void Check(Rules rules, int? level, object? value, StackTrace? putAt)
        {
            switch (value)
            {
                case Enum choice:
                    CheckChoice(rules, level, choice, putAt);
                    break;
                case int whole:
                    CheckNumber(rules, level, whole, whole.ToString(CultureInfo.InvariantCulture), string.Empty, putAt);
                    break;
                case float real:
                    CheckNumber(rules, level, real, real.ToString(CultureInfo.InvariantCulture), string.Empty, putAt);
                    break;
                case string text:
                    CheckText(rules, level, text, putAt);
                    break;
                case null:
                    if (rules.Required)
                    {
                        throw rules.Error("null", "any text that is not empty", level, putAt);
                    }

                    break;
                case IObjectList list:
                    CheckEntries(rules, level, list.Count, putAt);
                    break;
            }
        }

        private static void CheckNumber(Rules rules, int? level, double number, string shown, string each, StackTrace? putAt)
        {
            if (rules.NonNegative && number < 0)
            {
                var allowed = rules.Lifted ? "0 or more, even with [BeyondLimits]"
                    : rules.HasLimits ? Range(rules.Min, rules.Max) + each
                    : "0 or more";
                throw rules.Error(shown, allowed, level, putAt);
            }

            if (rules.HasLimits && !rules.Lifted && !(number >= rules.Min && number <= rules.Max))
            {
                throw rules.Error(shown, Range(rules.Min, rules.Max) + each + Lift, level, putAt);
            }
        }

        private static void CheckText(Rules rules, int? level, string text, StackTrace? putAt)
        {
            var shown = "\"" + text + "\"";
            if (rules.Required && text.Length == 0)
            {
                throw rules.Error(shown, "any text that is not empty", level, putAt);
            }

            if (rules.MaxLength.HasValue && !rules.Lifted && text.Length > rules.MaxLength.Value)
            {
                throw rules.Error(shown + " (" + Count(text.Length) + " characters)", "at most " + Count(rules.MaxLength.Value) + " characters" + Lift, level, putAt);
            }

            var entries = text.Length == 0 ? new string[0] : text.Split(',');
            if (rules.MaxEntries.HasValue && !rules.Lifted && entries.Length > rules.MaxEntries.Value)
            {
                throw rules.Error(shown + " (" + Count(entries.Length) + " entries)", "at most " + Count(rules.MaxEntries.Value) + " entries" + Lift, level, putAt);
            }

            if (rules.HasLimits || rules.NonNegative)
            {
                // A number list held as text (intList, unrealList): each number is checked; a part that is no number is left to the game.
                foreach (var entry in entries)
                {
                    if (double.TryParse(entry.Trim(), NumberStyles.Float, CultureInfo.InvariantCulture, out var number))
                    {
                        CheckNumber(rules, level, number, shown, " for each number", putAt);
                    }
                }
            }
        }

        private static void CheckChoice(Rules rules, int? level, Enum choice, StackTrace? putAt)
        {
            var type = choice.GetType();
            var number = Convert.ToInt64(choice, CultureInfo.InvariantCulture);
            var flags = type.IsDefined(typeof(FlagsAttribute), false);
            bool known;
            if (flags)
            {
                long all = 0;
                foreach (var member in Enum.GetValues(type))
                {
                    all |= Convert.ToInt64(member, CultureInfo.InvariantCulture);
                }

                known = (number & ~all) == 0;
            }
            else
            {
                known = Enum.IsDefined(type, choice);
            }

            if (!known)
            {
                var allowed = "only the values of " + type.Name + (flags ? " and their combinations" : string.Empty);
                throw rules.Error(number.ToString(CultureInfo.InvariantCulture), allowed, level, putAt);
            }
        }

        private static string Range(double min, double max)
        {
            if (double.IsNegativeInfinity(min))
            {
                return "at most " + Number(max);
            }

            if (double.IsPositiveInfinity(max))
            {
                return Number(min) + " or more";
            }

            return Number(min) + " to " + Number(max);
        }

        private static string Number(double number)
        {
            return number.ToString("R", CultureInfo.InvariantCulture);
        }

        private static string Count(int count)
        {
            return count.ToString(CultureInfo.InvariantCulture);
        }

        private static Rules? RulesOf(Type type, string field)
        {
            lock (rulesByClass)
            {
                if (!rulesByClass.TryGetValue(type, out var fields))
                {
                    rulesByClass[type] = fields = new Dictionary<string, object?>(StringComparer.Ordinal);
                }

                if (!fields.TryGetValue(field, out var rules))
                {
                    fields[field] = rules = ReadRules(type, field);
                }

                return (Rules?)rules;
            }
        }

        /// <summary>The rules of the property of <paramref name="field"/> in <paramref name="type"/>'s base chain; null when no property is the field.</summary>
        private static Rules? ReadRules(Type type, string field)
        {
            for (var parent = type; parent != null; parent = parent.BaseType)
            {
                foreach (var property in parent.GetProperties(BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.DeclaredOnly))
                {
                    var attribute = property.GetCustomAttribute<FieldAttribute>(false);
                    if (attribute != null && attribute.Id == field)
                    {
                        return new Rules(ClassName(type), property, attribute, Lifted(type, property.Name));
                    }
                }
            }

            return null;
        }

        /// <summary>True when <see cref="BeyondLimitsAttribute"/> on the class or a base class names the property.</summary>
        private static bool Lifted(Type type, string property)
        {
            foreach (BeyondLimitsAttribute lift in type.GetCustomAttributes(typeof(BeyondLimitsAttribute), true))
            {
                if (Array.IndexOf(lift.Properties, property) >= 0)
                {
                    return true;
                }
            }

            return false;
        }

        /// <summary>The class's name as a modder writes it, without its namespace: <c>Arthas</c>, <c>Heroes.Arthas</c> for a nested class.</summary>
        private static string ClassName(Type type)
        {
            var name = type.Name;
            for (var outer = type.DeclaringType; outer != null; outer = outer.DeclaringType)
            {
                name = outer.Name + "." + name;
            }

            return name;
        }

        /// <summary>The rules of one field as one class has them.</summary>
        /// @CSharpLua.Ignore
        private sealed class Rules
        {
            private readonly string property;
            private readonly string fieldId;
            private readonly string label;

            public Rules(string className, PropertyInfo info, FieldAttribute field, bool lifted)
            {
                property = className + "." + info.Name;
                fieldId = field.Id;
                label = field.Label;
                Lifted = lifted;
                var limits = info.GetCustomAttribute<LimitsAttribute>(false);
                HasLimits = limits != null;
                Min = limits == null ? double.NegativeInfinity : limits.Min;
                Max = limits == null ? double.PositiveInfinity : limits.Max;
                NonNegative = info.IsDefined(typeof(NonNegativeAttribute), false);
                Required = info.IsDefined(typeof(RequiredAttribute), false);
                MaxLength = info.GetCustomAttribute<MaxLengthAttribute>(false)?.Length;
                MaxEntries = info.GetCustomAttribute<MaxEntriesAttribute>(false)?.Count;
            }

            public bool Lifted { get; }

            public bool HasLimits { get; }

            public double Min { get; }

            public double Max { get; }

            public bool NonNegative { get; }

            public bool Required { get; }

            public int? MaxLength { get; }

            public int? MaxEntries { get; }

            public FieldRuleException Error(string value, string allowed, int? level, StackTrace? putAt)
            {
                return new FieldRuleException(property, fieldId, label, value, allowed, level) { PutAt = putAt };
            }
        }
    }
}
