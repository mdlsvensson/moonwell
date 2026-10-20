using System;

namespace Moonwell.Objects
{
    /// <summary>Marks a property of a kind base as the object field with this id (such as <c>uhpm</c>).</summary>
    /// @CSharpLua.Ignore
    [AttributeUsage(AttributeTargets.Property, AllowMultiple = false, Inherited = false)]
    public sealed class FieldAttribute : Attribute
    {
        /// <summary>The id of the field.</summary>
        public string Id { get; }

        /// <summary>The field's World Editor label (<c>Hit Points Maximum (Base)</c>), which errors name; empty when it has none.</summary>
        public string Label { get; set; } = string.Empty;

        /// <summary>Marks a property as the field with this id.</summary>
        public FieldAttribute(string id)
        {
            Id = id;
        }
    }
}
