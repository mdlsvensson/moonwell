using System;

namespace Moonwell.Objects
{
    /// <summary>Gives a declared class the id of its object in the map, as World Editor shows it; every new object needs one.</summary>
    /// @CSharpLua.Ignore
    [AttributeUsage(AttributeTargets.Class, AllowMultiple = false, Inherited = false)]
    public sealed class ObjectIdAttribute : Attribute
    {
        /// <summary>The id of the object.</summary>
        public string Id { get; }

        /// <summary>The id of a buff's carrier: the hidden ability that shows the buff's icon. Every declared buff needs one.</summary>
        public string? Carrier { get; set; }

        /// <summary>Gives a declared class this object id.</summary>
        public ObjectIdAttribute(string id)
        {
            Id = id;
        }
    }
}
