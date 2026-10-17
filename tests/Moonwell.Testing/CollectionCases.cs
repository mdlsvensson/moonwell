using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>Cases that check the ordered set and map.</summary>
    public static class CollectionCases
    {
        /// <summary>Makes the set 10, 3, 7, 1000, 42, -5, 99999, 0 in that order.</summary>
        private static OrderedSet<int> MakeSet()
        {
            var set = new OrderedSet<int>();
            set.Add(10);
            set.Add(3);
            set.Add(7);
            set.Add(1000);
            set.Add(42);
            set.Add(-5);
            set.Add(99999);
            set.Add(0);
            return set;
        }

        /// <summary>The ten keys the map cases use, in insertion order.</summary>
        private static string[] MapNames()
        {
            return new string[] { "alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta", "iota", "kappa" };
        }

        /// <summary>Makes the map "alpha".."kappa" set to 0..9 in that order.</summary>
        private static OrderedMap<string, int> MakeMap()
        {
            var names = MapNames();
            var map = new OrderedMap<string, int>();
            for (var i = 0; i < names.Length; i++)
            {
                map[names[i]] = i;
            }

            return map;
        }

        /// <summary>A set reads its items in insertion order, and a present item is not added again.</summary>
        public static void SetKeepsOrder(FakeWorld world)
        {
            var set = MakeSet();
            var expected = new int[] { 10, 3, 7, 1000, 42, -5, 99999, 0 };
            for (var i = 0; i < expected.Length; i++)
            {
                Expect.Equal(expected[i], set[i], "set item " + i);
            }

            Expect.True(!set.Add(7), "adding 7 again");
            Expect.Equal(8, set.Count, "count after adding 7 again");
        }

        /// <summary>Removing items from a set keeps the order of the rest, and a removed item is gone.</summary>
        public static void SetRemoveKeepsOrder(FakeWorld world)
        {
            var set = MakeSet();
            Expect.True(set.Remove(7), "removing 7");
            Expect.True(set.Remove(10), "removing 10");
            var expected = new int[] { 3, 1000, 42, -5, 99999, 0 };
            Expect.Equal(expected.Length, set.Count, "count after removing 7 and 10");
            for (var i = 0; i < expected.Length; i++)
            {
                Expect.Equal(expected[i], set[i], "set item after removing " + i);
            }

            Expect.True(!set.Contains(7), "contains 7 after removing it");
            Expect.True(!set.Remove(7), "removing 7 again");
        }

        /// <summary>A snapshot lets a loop remove every item of the set it reads from.</summary>
        public static void SetSnapshot(FakeWorld world)
        {
            var set = MakeSet();
            foreach (var item in set.Snapshot())
            {
                set.Remove(item);
            }

            Expect.Equal(0, set.Count, "count after removing every item from a snapshot");
        }

        /// <summary>A map reads its keys and values in insertion order, and setting a present key keeps its place.</summary>
        public static void MapKeepsOrder(FakeWorld world)
        {
            var map = MakeMap();
            var names = MapNames();
            for (var i = 0; i < names.Length; i++)
            {
                Expect.Equal(names[i], map.KeyAt(i), "key at " + i);
                Expect.Equal(i, map.ValueAt(i), "value at " + i);
            }

            map["delta"] = 30;
            Expect.Equal(10, map.Count, "count after setting delta again");
            Expect.Equal("delta", map.KeyAt(3), "key at 3 after setting delta again");
            Expect.Equal(30, map.ValueAt(3), "value at 3 after setting delta again");
        }

        /// <summary>Removing keys from a map keeps the order of the rest, and a removed key is gone.</summary>
        public static void MapRemove(FakeWorld world)
        {
            var map = MakeMap();
            Expect.True(map.Remove("alpha"), "removing alpha");
            Expect.True(map.Remove("eta"), "removing eta");
            Expect.Equal(8, map.Count, "count after removing alpha and eta");
            Expect.Equal("beta", map.KeyAt(0), "first key after removing alpha");
            Expect.True(!map.TryGetValue("eta", out _), "TryGetValue of eta after removing it");
            Expect.Throws<KeyNotFoundException>(() => { var unused = map["eta"]; }, "reading map[\"eta\"] after removing it");
        }
    }
}
