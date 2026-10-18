using System;

namespace Moonwell.Objects
{
    /// <summary>Gives a declared class the id it keeps for its object in the map.</summary>
    /// @CSharpLua.Ignore
    [AttributeUsage(AttributeTargets.Class, AllowMultiple = false, Inherited = false)]
    public sealed class ObjectIdAttribute : Attribute
    {
        /// <summary>The id of the object.</summary>
        public string Id { get; }

        /// <summary>Gives a declared class this object id.</summary>
        public ObjectIdAttribute(string id)
        {
            Id = id;
        }
    }
}
