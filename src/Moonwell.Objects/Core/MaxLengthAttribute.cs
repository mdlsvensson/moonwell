using System;

namespace Moonwell.Objects
{
    /// <summary>The most characters World Editor allows a text field; <see cref="BeyondLimitsAttribute"/> lifts it.</summary>
    /// @CSharpLua.Ignore
    [AttributeUsage(AttributeTargets.Property, AllowMultiple = false, Inherited = false)]
    public sealed class MaxLengthAttribute : Attribute
    {
        /// <summary>The most characters allowed.</summary>
        public int Length { get; }

        /// <summary>Marks a property with the most characters allowed.</summary>
        public MaxLengthAttribute(int length)
        {
            Length = length;
        }
    }
}
