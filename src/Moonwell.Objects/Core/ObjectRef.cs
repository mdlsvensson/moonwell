using System;

namespace Moonwell.Objects
{
    /// <summary>A declared class a field points to, or none.</summary>
    /// @CSharpLua.Ignore
    public sealed class ObjectRef<T>
    {
        // A Type, or the id text of a standard object that has no standard class of the field's kind.
        private object? entry;

        /// <summary>The class the field points to; null when none was set, or when it points to an id.</summary>
        internal Type? Type
        {
            get { return entry as Type; }
        }

        /// <summary>What the field points to: a <see cref="System.Type"/>, the id text of a standard object with no class, or null.</summary>
        internal object? Entry
        {
            get { return entry; }
            set { entry = value; }
        }

        /// <summary>Points the field to this class.</summary>
        public void Set<TObject>()
            where TObject : T
        {
            entry = typeof(TObject);
        }

        /// <summary>Points the field to the standard object of an id with no standard class of the field's kind, as a standard class's constructor writes it.</summary>
        internal void Set(string id)
        {
            entry = id;
        }

        /// <summary>Whether the field points to this class.</summary>
        public bool Is<TObject>()
            where TObject : T
        {
            return Type == typeof(TObject);
        }
    }
}
