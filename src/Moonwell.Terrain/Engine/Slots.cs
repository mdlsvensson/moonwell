namespace Moonwell.Engine
{
    /// <summary>
    /// Reads of the grids' arrays for the reads of the ground, which in Lua skip what CoreSystem's arrays check. In Lua an
    /// array keeps the item at index i in the table's slot i + 1, and a null item is the System.null sentinel. These go to
    /// the slots directly: no bounds check and no call. So they are only for an index inside the array. .NET runs the
    /// bodies: the array's own reads.
    /// </summary>
    internal static class Slots
    {
        // Raw: the slot of an array item in Lua. Safe: the callers' index is inside the array, and the item is a number or
        // a block that is there (Has said so).
        /// <summary>The item at an index inside an array.</summary>
        /// @CSharpLua.Template = "{0}[({1}) + 1]"
        public static T At<T>(T[] array, int index)
        {
            return array[index];
        }

        // Raw: a slot of an array of blocks in Lua, which holds the System.null sentinel for a null. Safe: the index is
        // inside the array.
        /// <summary>Whether the block at an index inside an array of blocks is there, not null.</summary>
        /// @CSharpLua.Template = "({0}[({1}) + 1] ~= System.null)"
        public static bool Has<T>(T[] array, int index)
            where T : class?
        {
            return array[index] != null;
        }

        // Raw: math.floor in place of CSharp.lua's System.ToInt32 and MathF.Floor, three calls; Lua 5.3 gives an integer
        // for a float that has one. Safe: the callers' value is kept from 0 to a grid's last corner or cell first, so it is
        // 0 or more and never NaN, where floor and truncation agree.
        /// <summary>The whole part of a float of 0 or more.</summary>
        /// @CSharpLua.Template = "math.floor({0})"
        public static int Whole(float value)
        {
            return (int)value;
        }
    }
}
