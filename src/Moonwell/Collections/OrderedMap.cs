using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>A map that keeps its keys in insertion order, so a loop gives the same order on every machine.</summary>
    public sealed class OrderedMap<TKey, TValue> where TKey : notnull
    {
        private readonly List<TKey> keys = new List<TKey>();
        private readonly Dictionary<TKey, TValue> values = new Dictionary<TKey, TValue>();

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
        }
    }
}
