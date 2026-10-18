using System;

namespace Moonwell.Objects
{
    /// <summary>A declared class a field points to, or none.</summary>
    /// @CSharpLua.Ignore
    public sealed class ObjectRef<T>
    {
        /// <summary>The class the field points to; null when none was set.</summary>
        internal Type? Type { get; private set; }

        /// <summary>Points the field to this class.</summary>
        public void Set<TObject>()
            where TObject : T
        {
            Type = typeof(TObject);
        }

        /// <summary>Whether the field points to this class.</summary>
        public bool Is<TObject>()
            where TObject : T
        {
            return Type == typeof(TObject);
        }
    }
}
