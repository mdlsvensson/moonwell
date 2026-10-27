using System.Collections;
using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>
    /// A set that keeps its items in insertion order, so a loop gives the same order on every machine: <c>foreach (var item in set)</c>,
    /// LINQ, or by index. A loop that adds or removes an item throws; to change the set as you go, loop over <see cref="Snapshot"/>.
    /// </summary>
    [LocalSafe]
    public sealed class OrderedSet<T> : IEnumerable<T> where T : notnull
    {
        private readonly List<T> items = new List<T>();
        private readonly HashSet<T> lookup = new HashSet<T>();

        /// <summary>How many items the set holds.</summary>
        public int Count
        {
            get
            {
                return items.Count;
            }
        }

        /// <summary>The item at an index, in insertion order.</summary>
        public T this[int index]
        {
            get
            {
                return items[index];
            }
        }

        /// <summary>Adds an item at the end; false when it is already present, and then it keeps its place.</summary>
        public bool Add(T item)
        {
            if (!lookup.Add(item))
            {
                return false;
            }

            items.Add(item);
            return true;
        }

        /// <summary>Removes an item; false when it is not present.</summary>
        public bool Remove(T item)
        {
            if (!lookup.Remove(item))
            {
                return false;
            }

            items.Remove(item);
            return true;
        }

        /// <summary>Whether the item is in the set.</summary>
        public bool Contains(T item)
        {
            return lookup.Contains(item);
        }

        /// <summary>Removes every item.</summary>
        public void Clear()
        {
            items.Clear();
            lookup.Clear();
        }

        /// <summary>A new list of the items in order, for a loop that may change the set.</summary>
        public List<T> Snapshot()
        {
            return new List<T>(items);
        }

        /// <summary>The items in insertion order, for foreach; adding or removing an item during the loop throws an InvalidOperationException.</summary>
        public List<T>.Enumerator GetEnumerator()
        {
            return items.GetEnumerator();
        }

        /// <summary>The items in insertion order, for LINQ and whatever takes an IEnumerable.</summary>
        IEnumerator<T> IEnumerable<T>.GetEnumerator()
        {
            return items.GetEnumerator();
        }

        /// <summary>The items in insertion order.</summary>
        IEnumerator IEnumerable.GetEnumerator()
        {
            return items.GetEnumerator();
        }
    }
}
