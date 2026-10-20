using System;

namespace Moonwell.Objects
{
    /// <summary>
    /// Marks a number field that allows no value below 0: its lowest allowed value is 0 or more, or it has none and the
    /// metadata forbids negatives. <see cref="BeyondLimitsAttribute"/> never lifts it.
    /// </summary>
    /// @CSharpLua.Ignore
    [AttributeUsage(AttributeTargets.Property, AllowMultiple = false, Inherited = false)]
    public sealed class NonNegativeAttribute : Attribute
    {
    }
}
