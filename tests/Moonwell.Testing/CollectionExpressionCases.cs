using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>
    /// Cases for collection expressions ([1, 2]): CSharp.lua dropped one in a return or a field and stopped on one in a local, an
    /// assignment or an argument (C1).
    /// </summary>
    public static class CollectionExpressionCases
    {
        private static readonly List<int> Field = [1, 2];

        /// <summary>A collection expression in a return, a field, a local, an assignment and an argument has its items.</summary>
        public static void Everywhere(FakeWorld world)
        {
            Expect.Equal(3, Returned().Length, "in a return");
            Expect.Equal(2, Field.Count, "in a field");
            List<int> local = [4];
            Expect.Equal(1, local.Count, "in a local");
            local = [5, 6, 7];
            Expect.Equal(3, local.Count, "in an assignment");
            Expect.Equal(2, Sum([8, 9]) - 15, "in an argument");
            int[] empty = [];
            Expect.Equal(0, empty.Length, "an empty one");
        }

        /// <summary>A spread adds the items of another collection in place.</summary>
        public static void Spread(FakeWorld world)
        {
            List<int> first = [1, 2];
            List<int> joined = [..first, 3, ..first];
            Expect.Equal(5, joined.Count, "the count of a spread");
            Expect.Equal(3, joined[2], "an item after a spread");
            Expect.Equal(2, joined[4], "an item of the second spread");
        }

        private static int[] Returned()
        {
            return [1, 2, 3];
        }

        private static int Sum(IEnumerable<int> items)
        {
            var total = 0;
            foreach (var item in items)
            {
                total += item;
            }

            return total;
        }
    }
}
