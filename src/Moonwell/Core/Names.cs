namespace Moonwell
{
    /// <summary>The class and the class name of an object, in .NET and in Lua.</summary>
    internal static class Names
    {
        // In Lua the class of an object is its metatable; .NET has GetType for it.
        /// @CSharpLua.Template = "getmetatable({0})"
        internal static object ClassOf(object obj)
        {
            return obj.GetType();
        }

        // In Lua the class name is the name of the class's System type.
        /// @CSharpLua.Template = "System.typeof(getmetatable({0})):getName()"
        internal static string NameOf(object obj)
        {
            return obj.GetType().Name;
        }
    }
}
