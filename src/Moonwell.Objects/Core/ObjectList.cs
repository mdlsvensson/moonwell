using System;
using System.Collections.Generic;
using System.Linq;

namespace Moonwell.Objects
{
    /// <summary>A list of declared classes a field holds, in order, each once; it starts as the standard object's list.</summary>
    /// @CSharpLua.Ignore
    public sealed class ObjectList<T> : IObjectList
    {
        // Each entry is a Type, or the id text of a standard object that has no standard class.
        private readonly List<object> entries;
        private readonly object[] standard;

        /// <summary>Called with the number of entries after the list grew, to check it against its field's rules; null for none.</summary>
        internal Action<int>? Grew { get; set; }

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
                Grew?.Invoke(entries.Count);
            }
        }

        /// <summary>
        /// Adds the standard object of an id with no standard class of the list's kind to the end; one already there stays
        /// where it is. A standard class's constructor writes such an entry so; a map's class names classes.
        /// </summary>
        internal void Add(string id)
        {
            if (!entries.Contains(id))
            {
                entries.Add(id);
                Grew?.Invoke(entries.Count);
            }
        }

        /// <summary>Takes a class out of the list; one not there changes nothing.</summary>
        public void Remove<TObject>()
            where TObject : T
        {
            entries.Remove(typeof(TObject));
        }

        /// <summary>Replaces the whole list with this class.</summary>
        public void Set<T1>()
            where T1 : T
        {
            SetTo(typeof(T1));
        }

        /// <summary>Replaces the whole list with these classes, in order; a class given twice is kept at its first place.</summary>
        public void Set<T1, T2>()
            where T1 : T
            where T2 : T
        {
            SetTo(typeof(T1), typeof(T2));
        }

        /// <summary>Replaces the whole list with these classes, in order; a class given twice is kept at its first place.</summary>
        public void Set<T1, T2, T3>()
            where T1 : T
            where T2 : T
            where T3 : T
        {
            SetTo(typeof(T1), typeof(T2), typeof(T3));
        }

        /// <summary>Replaces the whole list with these classes, in order; a class given twice is kept at its first place.</summary>
        public void Set<T1, T2, T3, T4>()
            where T1 : T
            where T2 : T
            where T3 : T
            where T4 : T
        {
            SetTo(typeof(T1), typeof(T2), typeof(T3), typeof(T4));
        }

        /// <summary>Replaces the whole list with these classes, in order; a class given twice is kept at its first place.</summary>
        public void Set<T1, T2, T3, T4, T5>()
            where T1 : T
            where T2 : T
            where T3 : T
            where T4 : T
            where T5 : T
        {
            SetTo(typeof(T1), typeof(T2), typeof(T3), typeof(T4), typeof(T5));
        }

        /// <summary>Replaces the whole list with these classes, in order; a class given twice is kept at its first place.</summary>
        public void Set<T1, T2, T3, T4, T5, T6>()
            where T1 : T
            where T2 : T
            where T3 : T
            where T4 : T
            where T5 : T
            where T6 : T
        {
            SetTo(typeof(T1), typeof(T2), typeof(T3), typeof(T4), typeof(T5), typeof(T6));
        }

        /// <summary>Replaces the whole list with these classes, in order; a class given twice is kept at its first place.</summary>
        public void Set<T1, T2, T3, T4, T5, T6, T7>()
            where T1 : T
            where T2 : T
            where T3 : T
            where T4 : T
            where T5 : T
            where T6 : T
            where T7 : T
        {
            SetTo(typeof(T1), typeof(T2), typeof(T3), typeof(T4), typeof(T5), typeof(T6), typeof(T7));
        }

        /// <summary>Replaces the whole list with these classes, in order; a class given twice is kept at its first place.</summary>
        public void Set<T1, T2, T3, T4, T5, T6, T7, T8>()
            where T1 : T
            where T2 : T
            where T3 : T
            where T4 : T
            where T5 : T
            where T6 : T
            where T7 : T
            where T8 : T
        {
            SetTo(typeof(T1), typeof(T2), typeof(T3), typeof(T4), typeof(T5), typeof(T6), typeof(T7), typeof(T8));
        }

        /// <summary>Puts a class in the place of another; with the other absent nothing changes, and with the class already there the other is only taken out.</summary>
        public void Replace<TOld, TNew>()
            where TOld : T
            where TNew : T
        {
            var at = entries.IndexOf(typeof(TOld));
            if (at < 0)
            {
                return;
            }

            if (entries.Contains(typeof(TNew)))
            {
                entries.RemoveAt(at);
                return;
            }

            entries[at] = typeof(TNew);
        }

        private void SetTo(params Type[] types)
        {
            entries.Clear();
            foreach (var type in types)
            {
                if (!entries.Contains(type))
                {
                    entries.Add(type);
                }
            }

            Grew?.Invoke(entries.Count);
        }

        /// <summary>Whether the list holds this class.</summary>
        public bool Contains<TObject>()
            where TObject : T
        {
            return entries.Contains(typeof(TObject));
        }
    }

    /// <summary>An object list as the rule checks read it, whatever the type of its entries.</summary>
    /// @CSharpLua.Ignore
    internal interface IObjectList
    {
        /// <summary>The number of entries in the list.</summary>
        int Count { get; }
    }
}
