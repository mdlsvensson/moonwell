#if !MOONWELL_LUA

using System;

namespace Moonwell.Objects
{
    /// <summary>
    /// Lets a declared class put values past World Editor's limits on the properties it names
    /// (<c>[BeyondLimits(nameof(HitPoints))]</c>): <see cref="LimitsAttribute"/>, <see cref="MaxLengthAttribute"/> and
    /// <see cref="MaxEntriesAttribute"/>. It never lifts <see cref="NonNegativeAttribute"/>,
    /// <see cref="RequiredAttribute"/> or a choice that is no value of its enum. A class inherits the names its base
    /// classes give.
    /// </summary>
    /// @CSharpLua.Ignore
    [AttributeUsage(AttributeTargets.Class, AllowMultiple = true, Inherited = true)]
    public sealed class BeyondLimitsAttribute : Attribute
    {
        /// <summary>The names of the properties whose limits are lifted.</summary>
        public string[] Properties { get; }

        /// <summary>Lifts the limits of the properties with these names.</summary>
        public BeyondLimitsAttribute(params string[] properties)
        {
            Properties = properties;
        }
    }
}

#endif
