using System;

namespace Moonwell.Objects
{
    /// <summary>Gives a declared class the id of its object in the map, as World Editor shows it; every new object needs one.</summary>
    /// @CSharpLua.Ignore
    [AttributeUsage(AttributeTargets.Class, AllowMultiple = false, Inherited = false)]
    public sealed class ObjectIdAttribute : Attribute
    {
        /// <summary>The id of the object; null for a class that replaces a standard object, which keeps its id.</summary>
        public string? Id { get; }

        /// <summary>The id of a buff's carrier: the hidden ability that shows the buff's icon. Every declared buff needs one.</summary>
        public string? Carrier { get; set; }

        /// <summary>Gives no id: for a buff that replaces a standard buff, which still names its <see cref="Carrier"/>.</summary>
        public ObjectIdAttribute()
        {
        }

        /// <summary>Gives a declared class this object id.</summary>
        public ObjectIdAttribute(string id)
        {
            Id = id;
        }
    }
}
