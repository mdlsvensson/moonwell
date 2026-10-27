using System;
using System.Collections;
using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>
    /// A map that keeps its keys in insertion order, so a loop gives the same order on every machine: <c>foreach (var pair in map)</c>
    /// gives each key and its value (<c>pair.Key</c>, <c>pair.Value</c>), and LINQ works too. A loop that adds or removes a key
    /// throws; setting a present key's value does not. In the game each pair is a small table: in code that runs every tick over
    /// many keys, loop by index with <see cref="KeyAt"/> and <see cref="ValueAt"/>, which makes none.
    /// </summary>
    [LocalSafe]
    public sealed class OrderedMap<TKey, TValue> : IEnumerable<KeyValuePair<TKey, TValue>> where TKey : notnull
    {
        private readonly List<TKey> keys = new List<TKey>();
        private readonly Dictionary<TKey, TValue> values = new Dictionary<TKey, TValue>();

        /// <summary>Counts the keys added and removed, so a loop over the map can tell that it changed.</summary>
        private int version;

        /// <summary>How many keys the map holds.</summary>
        public int Count
        {
            get
            {
                return keys.Count;
            }
        }

        /// <summary>The value for a key; a new key is added at the end, and a present key gets its value in place.</summary>
        public TValue this[TKey key]
        {
            get
            {
                if (!values.TryGetValue(key, out var value))
                {
                    throw new KeyNotFoundException("The map has no such key.");
                }

                return value;
            }
            set
            {
                if (!values.ContainsKey(key))
                {
                    keys.Add(key);
                    version++;
                }

                values[key] = value;
            }
        }

        /// <summary>Gets the value for a key; false when the key is not present.</summary>
        public bool TryGetValue(TKey key, out TValue value)
        {
            return values.TryGetValue(key, out value!);
        }

        /// <summary>Whether the key is in the map.</summary>
        public bool ContainsKey(TKey key)
        {
            return values.ContainsKey(key);
        }

        /// <summary>Removes a key and its value; false when the key is not present.</summary>
        public bool Remove(TKey key)
        {
            if (!values.Remove(key))
            {
                return false;
            }

            keys.Remove(key);
            version++;
            return true;
        }

        /// <summary>The key at an index, in insertion order.</summary>
        public TKey KeyAt(int index)
        {
            return keys[index];
        }

        /// <summary>The value at an index, in insertion order.</summary>
        public TValue ValueAt(int index)
        {
            return values[keys[index]];
        }

        /// <summary>Removes every key and value.</summary>
        public void Clear()
        {
            keys.Clear();
            values.Clear();
            version++;
        }

        /// <summary>The keys and their values in insertion order, for foreach; adding or removing a key during the loop throws an InvalidOperationException.</summary>
        public Enumerator GetEnumerator()
        {
            return new Enumerator(this);
        }

        /// <summary>The keys and their values in insertion order, for LINQ and whatever takes an IEnumerable.</summary>
        IEnumerator<KeyValuePair<TKey, TValue>> IEnumerable<KeyValuePair<TKey, TValue>>.GetEnumerator()
        {
            return new Enumerator(this);
        }

        /// <summary>The keys and their values in insertion order.</summary>
        IEnumerator IEnumerable.GetEnumerator()
        {
            return new Enumerator(this);
        }

        /// <summary>
        /// Goes through a map's keys and their values in insertion order; foreach uses it. It reads each key by its index, as
        /// <see cref="KeyAt"/> does: CSharp.lua has no Lua type for a field holding a List's own enumerator.
        /// </summary>
        public struct Enumerator : IEnumerator<KeyValuePair<TKey, TValue>>
        {
            private readonly OrderedMap<TKey, TValue> map;
            private readonly int version;
            private int index;

            internal Enumerator(OrderedMap<TKey, TValue> map)
            {
                this.map = map;
                version = map.version;
                index = -1;
            }

            /// <summary>The key the loop is at, with its value as the map holds it now.</summary>
            public KeyValuePair<TKey, TValue> Current
            {
                get
                {
                    var key = map.keys[index];
                    return new KeyValuePair<TKey, TValue>(key, map.values[key]);
                }
            }

            /// <summary>The key the loop is at, with its value.</summary>
            object IEnumerator.Current
            {
                get
                {
                    return Current;
                }
            }

            /// <summary>Moves to the next key; false after the last. Throws an InvalidOperationException when a key was added or removed since the loop began.</summary>
            public bool MoveNext()
            {
                if (version != map.version)
                {
                    throw new InvalidOperationException("The map was changed during the loop: a key was added or removed.");
                }

                index++;
                return index < map.keys.Count;
            }

            /// <summary>Goes back to before the first key.</summary>
            public void Reset()
            {
                index = -1;
            }

            /// <summary>Does nothing: the enumerator holds nothing to free.</summary>
            public void Dispose()
            {
            }
        }
    }
}
