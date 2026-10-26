using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>
    /// Reads and writes of the package's own lists for the cycle's inner loops, which in Lua skip what CoreSystem's List
    /// checks. In Lua a List keeps the item at index i in the table's slot i + 1, its count is the table's length, a null
    /// item is the System.null sentinel, and each change counts up a version that only an enumerator reads. These go to the
    /// slots directly: no bounds check, no sentinel, no version. So they are only for a list that never holds null, at an
    /// index below its count, and that no foreach walks (none of the package's lists is enumerated). .NET runs the bodies:
    /// the List's own members.
    /// </summary>
    internal static class Raw
    {
        // Raw: the slot of a List item in Lua. Safe: the callers' lists never hold null and the index is below the count.
        /// <summary>The item at an index of a list that holds no null, the index below the count.</summary>
        /// @CSharpLua.Template = "{0}[{1} + 1]"
        public static T At<T>(List<T> list, int index)
        {
            return list[index];
        }

        // Raw: a write to the slot of a List item in Lua, with no version counted. Safe: the item is not null and the index
        // is below the count, so the table keeps no hole.
        /// <summary>Sets the item at an index below the count to an item that is not null.</summary>
        /// @CSharpLua.Template = "{0}[{1} + 1] = {2}"
        public static void Put<T>(List<T> list, int index, T item)
        {
            list[index] = item;
        }

        // Raw: an append to the slot after the last in Lua, with no version counted. Safe: the item is not null, so the
        // table's length stays the count.
        /// <summary>Adds an item that is not null at the end of a list.</summary>
        /// @CSharpLua.Template = "{0}[#{0} + 1] = {1}"
        public static void Append<T>(List<T> list, T item)
        {
            list.Add(item);
        }

        // Raw: the slots past the count cleared from the last down, so the table's length is the count at every point.
        // Safe: no foreach walks the list, so no version needs counting.
        /// <summary>Takes the items from an index to the end off a list; a list no longer than that is left as it is.</summary>
        /// @CSharpLua.Template = "for rawSlot = #{0}, {1} + 1, -1 do {0}[rawSlot] = nil end"
        public static void Truncate<T>(List<T> list, int count)
        {
            while (list.Count > count)
            {
                list.RemoveAt(list.Count - 1);
            }
        }

        // Raw: in Lua an object is its own table, and System.cast checks its class through a call. Safe: the caller knows
        // the class, as an entry's kind says what its thing is.
        /// <summary>An object as the class the caller knows it is.</summary>
        /// @CSharpLua.Template = "{0}"
        public static T As<T>(object thing)
            where T : class
        {
            return (T)thing;
        }

        // Raw: math.floor in place of CSharp.lua's System.ToInt32, which truncates through two calls; Lua 5.3 gives an
        // integer for a float that has one. Safe: the value is 0 or more, where floor and truncation agree, or NaN (a place
        // a map set to 0/0), which gives what System.ToInt32 gives, so a NaN place fails as it did. The value is a local.
        /// <summary>The whole part of a float of 0 or more.</summary>
        /// @CSharpLua.Template = "({0} == {0} and math.floor({0}) or -2147483648)"
        public static int Whole(float value)
        {
            return (int)value;
        }
    }
}
