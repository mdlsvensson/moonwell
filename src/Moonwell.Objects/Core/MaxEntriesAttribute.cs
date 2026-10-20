using System;

namespace Moonwell.Objects
{
    /// <summary>
    /// The most entries World Editor allows a list field: the classes of an object list, or the comma-separated entries
    /// of a text list. <see cref="BeyondLimitsAttribute"/> lifts it.
    /// </summary>
    /// @CSharpLua.Ignore
    [AttributeUsage(AttributeTargets.Property, AllowMultiple = false, Inherited = false)]
    public sealed class MaxEntriesAttribute : Attribute
    {
        /// <summary>The most entries allowed.</summary>
        public int Count { get; }

        /// <summary>Marks a property with the most entries allowed.</summary>
        public MaxEntriesAttribute(int count)
        {
            Count = count;
        }
    }
}
