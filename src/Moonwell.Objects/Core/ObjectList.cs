using System;
using System.Collections.Generic;

namespace Moonwell.Objects
{
    /// <summary>A list of declared classes a field holds, in the order they were added, each once.</summary>
    /// @CSharpLua.Ignore
    public sealed class ObjectList<T>
    {
        private readonly List<Type> types = new List<Type>();

        /// <summary>The number of classes in the list.</summary>
        public int Count
        {
            get { return types.Count; }
        }

        /// <summary>The classes in the list, in order.</summary>
        internal IReadOnlyList<Type> Types
        {
            get { return types; }
        }

        /// <summary>Adds a class to the end of the list; one already there stays where it is.</summary>
        public void Add<TObject>()
            where TObject : T
        {
            if (!types.Contains(typeof(TObject)))
            {
                types.Add(typeof(TObject));
            }
        }

        /// <summary>Takes a class out of the list; one not there changes nothing.</summary>
        public void Remove<TObject>()
            where TObject : T
        {
            types.Remove(typeof(TObject));
        }

        /// <summary>Whether the list holds this class.</summary>
        public bool Contains<TObject>()
            where TObject : T
        {
            return types.Contains(typeof(TObject));
        }
    }
}
