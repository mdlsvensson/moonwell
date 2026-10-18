using System;

namespace Moonwell.Objects
{
    /// <summary>Makes a declared class change every object of its base type instead of adding a new one.</summary>
    /// @CSharpLua.Ignore
    [AttributeUsage(AttributeTargets.Class, AllowMultiple = false, Inherited = false)]
    public sealed class ReplacesAttribute : Attribute
    {
    }
}
