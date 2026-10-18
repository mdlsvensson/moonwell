using System;
using System.Collections.Generic;
using System.Linq;

namespace Moonwell.Objects
{
    /// <summary>A list of declared classes a field holds, in order, each once; it starts as the standard object's list.</summary>
    /// @CSharpLua.Ignore
    public sealed class ObjectList<T>
    {
        // Each entry is a Type, or the id text of a standard object that has no standard class.
        private readonly List<object> entries;
        private readonly object[] standard;

        /// <summary>Makes an empty list.</summary>
        public ObjectList()
            : this(Array.Empty<object>())
        {
        }

        /// <summary>Makes a list that starts with the standard object's entries.</summary>
        internal ObjectList(IReadOnlyList<object> standard)
        {
            this.standard = standard.ToArray();
            entries = new List<object>(this.standard);
        }

        /// <summary>The number of entries in the list.</summary>
        public int Count
        {
            get { return entries.Count; }
        }

        /// <summary>The classes in the list, in order; ids of standard objects with no class are left out.</summary>
        internal IReadOnlyList<Type> Types
        {
            get { return entries.OfType<Type>().ToList(); }
        }

        /// <summary>The entries in order: a <see cref="Type"/>, or the id text of a standard object with no class.</summary>
        internal IReadOnlyList<object> Entries
        {
            get { return entries; }
        }

        /// <summary>The entries the list started with.</summary>
        internal IReadOnlyList<object> Standard
        {
            get { return standard; }
        }

        /// <summary>Adds a class to the end of the list; one already there stays where it is.</summary>
        public void Add<TObject>()
            where TObject : T
        {
            if (!entries.Contains(typeof(TObject)))
            {
                entries.Add(typeof(TObject));
            }
        }

        /// <summary>Takes a class out of the list; one not there changes nothing.</summary>
        public void Remove<TObject>()
            where TObject : T
        {
            entries.Remove(typeof(TObject));
        }

        /// <summary>Whether the list holds this class.</summary>
        public bool Contains<TObject>()
            where TObject : T
        {
            return entries.Contains(typeof(TObject));
        }
    }
}
