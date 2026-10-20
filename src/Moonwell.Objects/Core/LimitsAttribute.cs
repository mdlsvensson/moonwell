using System;

namespace Moonwell.Objects
{
    /// <summary>
    /// The range World Editor allows a number field (its metadata's <c>minVal</c> and <c>maxVal</c>): a value put outside it
    /// is a build error, unless the class names the property in <see cref="BeyondLimitsAttribute"/>. A missing end is an
    /// infinity. For a number list held as text, each number of the list is checked.
    /// </summary>
    /// @CSharpLua.Ignore
    [AttributeUsage(AttributeTargets.Property, AllowMultiple = false, Inherited = false)]
    public sealed class LimitsAttribute : Attribute
    {
        /// <summary>The lowest value allowed; <see cref="double.NegativeInfinity"/> for no lower limit.</summary>
        public double Min { get; }

        /// <summary>The highest value allowed; <see cref="double.PositiveInfinity"/> for no upper limit.</summary>
        public double Max { get; }

        /// <summary>Marks a property with the range World Editor allows.</summary>
        public LimitsAttribute(double min, double max)
        {
            Min = min;
            Max = max;
        }
    }
}
