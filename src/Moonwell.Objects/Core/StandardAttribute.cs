using System;

namespace Moonwell.Objects
{
    /// <summary>Marks a class as the standard object of the game with this id (such as <c>Hpal</c>).</summary>
    /// @CSharpLua.Ignore
    [AttributeUsage(AttributeTargets.Class, AllowMultiple = false, Inherited = false)]
    public sealed class StandardAttribute : Attribute
    {
        /// <summary>The id of the standard object.</summary>
        public string Id { get; }

        /// <summary>Marks a class as the standard object with this id.</summary>
        public StandardAttribute(string id)
        {
            Id = id;
        }
    }
}
