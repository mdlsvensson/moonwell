#if !MOONWELL_LUA

using System;

namespace Moonwell.Objects
{
    /// <summary>Marks a text field that may not be put empty or null (an object's name). <see cref="BeyondLimitsAttribute"/> never lifts it.</summary>
    /// @CSharpLua.Ignore
    [AttributeUsage(AttributeTargets.Property, AllowMultiple = false, Inherited = false)]
    public sealed class RequiredAttribute : Attribute
    {
    }
}

#endif
