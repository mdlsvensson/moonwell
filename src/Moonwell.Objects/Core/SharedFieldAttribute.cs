#if !MOONWELL_LUA

using System;

namespace Moonwell.Objects
{
    /// <summary>
    /// Marks a class as having a shared field: a field whose live value is a member of the core class (<c>MaxMana</c> of a
    /// unit) and which the class has no property of its own for. It carries the rules a <see cref="FieldAttribute"/> property
    /// would carry (<see cref="LimitsAttribute"/>, <see cref="NonNegativeAttribute"/>, <see cref="RequiredAttribute"/>), so
    /// that a value put on the field is checked the same, and <c>[BeyondLimits(nameof(MaxMana))]</c> lifts it by
    /// <see cref="Member"/>. A property with the field's id wins over this attribute.
    /// </summary>
    /// @CSharpLua.Ignore
    [AttributeUsage(AttributeTargets.Class, AllowMultiple = true, Inherited = true)]
    public sealed class SharedFieldAttribute : Attribute
    {
        /// <summary>The id of the field.</summary>
        public string Id { get; }

        /// <summary>The name of the core member that is the field's live value, which errors and <see cref="BeyondLimitsAttribute"/> name.</summary>
        public string Member { get; }

        /// <summary>The field's World Editor label (<c>Mana Maximum</c>), which errors name; empty when it has none.</summary>
        public string Label { get; set; } = string.Empty;

        /// <summary>The lowest value allowed; read only when <see cref="HasLimits"/> is true.</summary>
        public double Min { get; set; } = double.NegativeInfinity;

        /// <summary>The highest value allowed; read only when <see cref="HasLimits"/> is true.</summary>
        public double Max { get; set; } = double.PositiveInfinity;

        /// <summary>True when the field has the range <see cref="Min"/> to <see cref="Max"/>, as <see cref="LimitsAttribute"/> gives a property.</summary>
        public bool HasLimits { get; set; }

        /// <summary>True when the field allows no negative value, as <see cref="NonNegativeAttribute"/> gives a property.</summary>
        public bool NonNegative { get; set; }

        /// <summary>True when the field allows no empty or null text, as <see cref="RequiredAttribute"/> gives a property.</summary>
        public bool Required { get; set; }

        /// <summary>Marks a class as having the shared field with this id, whose live value is the core member.</summary>
        public SharedFieldAttribute(string id, string member)
        {
            Id = id;
            Member = member;
        }
    }
}

#endif
