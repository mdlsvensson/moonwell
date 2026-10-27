using System;
using System.Collections.Generic;
using System.Linq;

namespace Moonwell.Testing
{
    /// <summary>Cases for the full audit's doc fixes: OrderedMap and OrderedSet looped with foreach and LINQ (DO4).</summary>
    public static class AuditDocsCases
    {
        /// <summary>A foreach over a set gives its items in insertion order, after adds and removes, and an empty set none.</summary>
        public static void SetForeachKeepsOrder(FakeWorld world)
        {
            var set = new OrderedSet<int>();
            set.Add(10);
            set.Add(3);
            set.Add(7);
            set.Add(3);
            set.Add(42);
            set.Remove(7);
            set.Add(-5);
            var seen = new List<int>();
            foreach (var item in set)
            {
                seen.Add(item);
            }

            Expect.Equal("10,3,42,-5", string.Join(",", seen), "the items a foreach gives");

            var empty = new OrderedSet<string>();
            foreach (var item in empty)
            {
                throw new ExpectationFailed("an empty set gave " + item);
            }
        }

        /// <summary>Removing an item inside a foreach over the set throws; a loop over its snapshot may remove every item.</summary>
        public static void SetForeachChangeThrows(FakeWorld world)
        {
            var set = new OrderedSet<int>();
            set.Add(1);
            set.Add(2);
            set.Add(3);
            Expect.Throws<InvalidOperationException>(() =>
            {
                foreach (var item in set)
                {
                    set.Remove(item);
                }
            }, "removing an item inside a foreach over the set");

            foreach (var item in set.Snapshot())
            {
                set.Remove(item);
            }

            Expect.Equal(0, set.Count, "count after removing every item from a snapshot");
        }

        /// <summary>
        /// A foreach over a map gives each key and its value in insertion order: a present key set again keeps its place and
        /// gives its new value, a removed key is gone, and the pair takes apart into a key and a value.
        /// </summary>
        public static void MapForeachKeepsOrder(FakeWorld world)
        {
            var map = new OrderedMap<string, int>();
            map["gamma"] = 3;
            map["alpha"] = 1;
            map["beta"] = 2;
            map["delta"] = 4;
            map["gamma"] = 30;
            map.Remove("beta");
            var seen = new List<string>();
            foreach (var pair in map)
            {
                seen.Add(pair.Key + "=" + pair.Value);
            }

            Expect.Equal("gamma=30,alpha=1,delta=4", string.Join(",", seen), "the pairs a foreach gives");

            var parts = new List<string>();
            foreach (var (key, value) in map)
            {
                parts.Add(key + value);
            }

            Expect.Equal("gamma30,alpha1,delta4", string.Join(",", parts), "the keys and values a foreach takes apart");

            var kept = new List<KeyValuePair<string, int>>();
            foreach (var pair in map)
            {
                kept.Add(pair);
            }

            Expect.Equal("alpha", kept[1].Key, "the second pair kept after the loop");
            Expect.Equal(1, kept[1].Value, "the second pair's value kept after the loop");

            var empty = new OrderedMap<int, int>();
            foreach (var pair in empty)
            {
                throw new ExpectationFailed("an empty map gave " + pair.Key);
            }
        }

        /// <summary>Adding or removing a key inside a foreach over the map throws; setting a present key's value does not.</summary>
        public static void MapForeachChangeThrows(FakeWorld world)
        {
            var map = new OrderedMap<string, int>();
            map["a"] = 1;
            map["b"] = 2;
            foreach (var pair in map)
            {
                map[pair.Key] = pair.Value * 10;
            }

            Expect.Equal(10, map["a"], "a after setting each value in the loop");
            Expect.Equal(20, map["b"], "b after setting each value in the loop");

            Expect.Throws<InvalidOperationException>(() =>
            {
                foreach (var pair in map)
                {
                    map[pair.Key + "!"] = 0;
                }
            }, "adding a key inside a foreach over the map");
            Expect.Throws<InvalidOperationException>(() =>
            {
                foreach (var pair in map)
                {
                    map.Remove(pair.Key);
                }
            }, "removing a key inside a foreach over the map");
        }

        /// <summary>A foreach over either left by break, continue or return, and one inside another over the same set.</summary>
        public static void ForeachLeftEarly(FakeWorld world)
        {
            var set = new OrderedSet<int>();
            for (var i = 9; i > 0; i--)
            {
                set.Add(i);
            }

            var before = new List<int>();
            foreach (var item in set)
            {
                if (item == 6)
                {
                    break;
                }

                if (item == 8)
                {
                    continue;
                }

                before.Add(item);
            }

            Expect.Equal("9,7", string.Join(",", before), "the items before the break, without the one continued");
            Expect.Equal(5, FirstBelow(set, 6), "the first item below 6, returned from inside the loop");

            var map = new OrderedMap<string, int>();
            map["x"] = 1;
            map["y"] = 2;
            map["z"] = 3;
            Expect.Equal("y", FirstKeyAbove(map, 1), "the first key above 1, returned from inside the loop");

            var small = new OrderedSet<int>();
            small.Add(1);
            small.Add(2);
            var pairs = new List<int>();
            foreach (var a in small)
            {
                foreach (var b in small)
                {
                    pairs.Add((a * 10) + b);
                }
            }

            Expect.Equal("11,12,21,22", string.Join(",", pairs), "a loop inside a loop over the same set");
        }

        /// <summary>LINQ over a set and a map reads them in insertion order.</summary>
        public static void Linq(FakeWorld world)
        {
            var set = new OrderedSet<string>();
            set.Add("c");
            set.Add("a");
            set.Add("b");
            Expect.Equal("c,b", string.Join(",", set.Where(item => item != "a")), "the set's items other than a");
            Expect.Equal("c", set.First(), "the set's first item");

            var map = new OrderedMap<string, int>();
            map["x"] = 1;
            map["y"] = 2;
            map["z"] = 3;
            Expect.Equal("y,z", string.Join(",", map.Where(pair => pair.Value > 1).Select(pair => pair.Key)), "the map's keys with a value above 1");
            Expect.Equal(2, map.Count(pair => pair.Value > 1), "how many values are above 1");
        }

        private static int FirstBelow(OrderedSet<int> set, int limit)
        {
            foreach (var item in set)
            {
                if (item < limit)
                {
                    return item;
                }
            }

            return -1;
        }

        private static string FirstKeyAbove(OrderedMap<string, int> map, int limit)
        {
            foreach (var pair in map)
            {
                if (pair.Value > limit)
                {
                    return pair.Key;
                }
            }

            return string.Empty;
        }
    }
}
