using System;

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

        // In Lua a type's table holds the class at [1], the same class ClassOf gives an object of that type.
        /// @CSharpLua.Template = "{0}[1]"
        internal static object ClassOfType(Type type)
        {
            return type;
        }

        // In Lua the System type of an object is its class's, the one typeof gives for the class; GetType is not there.
        /// @CSharpLua.Template = "System.typeof(getmetatable({0}))"
        internal static Type TypeOf(object obj)
        {
            return obj.GetType();
        }

        // In Lua the class name is the name of the class's System type.
        /// @CSharpLua.Template = "System.typeof(getmetatable({0})):getName()"
        internal static string NameOf(object obj)
        {
            return obj.GetType().Name;
        }

        /// <summary>The four characters of an object id, as World Editor shows them ("h00X"), the first from the highest byte; FourCC's reverse.</summary>
        internal static string IdText(int id)
        {
            return Character((id >> 24) & 255) + Character((id >> 16) & 255) + Character((id >> 8) & 255) + Character(id & 255);
        }

        /// <summary>The character of a code, as a string.</summary>
        private static string Character(int code)
        {
            return ((char)code).ToString();
        }
    }
}
