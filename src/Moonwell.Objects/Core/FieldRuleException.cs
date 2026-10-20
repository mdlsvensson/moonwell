using System;
using System.Diagnostics;

namespace Moonwell.Objects
{
    /// <summary>
    /// A value a declared class put that World Editor does not allow. The message is
    /// <c>{Property}[ at level {Level}] is {Value}; {Label} ({FieldId}) allows {Allowed}</c>, with the label and its
    /// parentheses left out when the field has no label: <c>Arthas.HitPoints is 600000; Hit Points Maximum (Base) (uhpm)
    /// allows 1 to 500000; [BeyondLimits] goes past it</c>. The build puts the file and line before it.
    /// </summary>
    /// @CSharpLua.Ignore
    public sealed class FieldRuleException : Exception
    {
        /// <summary>Makes the error of one value.</summary>
        /// <param name="property">The class and the property, as a modder writes them: <c>Arthas.HitPoints</c>.</param>
        /// <param name="fieldId">The field id: <c>uhpm</c>.</param>
        /// <param name="label">The field's World Editor label; empty when it has none.</param>
        /// <param name="value">The value as the message shows it: <c>600000</c>, <c>"text"</c>, <c>13 entries</c>.</param>
        /// <param name="allowed">What the field allows, as the message shows it after "allows".</param>
        /// <param name="level">The level of the value; null for a field without levels.</param>
        public FieldRuleException(string property, string fieldId, string label, string value, string allowed, int? level)
            : base(MessageOf(property, fieldId, label, value, allowed, level))
        {
            Property = property;
            FieldId = fieldId;
            Label = label;
            Value = value;
            Allowed = allowed;
            Level = level;
        }

        /// <summary>The class and the property: <c>Arthas.HitPoints</c>.</summary>
        public string Property { get; }

        /// <summary>The field id: <c>uhpm</c>.</summary>
        public string FieldId { get; }

        /// <summary>The field's World Editor label; empty when it has none.</summary>
        public string Label { get; }

        /// <summary>The value as the message shows it.</summary>
        public string Value { get; }

        /// <summary>What the field allows, as the message shows it after "allows": <c>1 to 500000; [BeyondLimits] goes past it</c>.</summary>
        public string Allowed { get; }

        /// <summary>The level of the value; null for a field without levels.</summary>
        public int? Level { get; }

        /// <summary>
        /// Where a formula of levels was put, for a value the formula gave when its levels were counted after the class's
        /// constructor ran (when no line of the class is on the stack); null for a value checked when it was put.
        /// </summary>
        public StackTrace? PutAt { get; internal set; }

        /// <summary>The message of an error: see <see cref="FieldRuleException"/>.</summary>
        public static string MessageOf(string property, string fieldId, string label, string value, string allowed, int? level)
        {
            var at = level.HasValue ? " at level " + level.Value.ToString(System.Globalization.CultureInfo.InvariantCulture) : string.Empty;
            var field = label.Length > 0 ? label + " (" + fieldId + ")" : fieldId;
            return property + at + " is " + value + "; " + field + " allows " + allowed;
        }
    }
}
