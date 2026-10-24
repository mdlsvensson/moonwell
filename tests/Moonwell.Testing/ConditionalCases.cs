namespace Moonwell.Testing
{
    /// <summary>
    /// Cases for <c>c ? a : b</c> whose first branch can be nil or false in Lua (C24): CSharp.lua wrote it <c>c and a or b</c>,
    /// which gives b there where C# gives a.
    /// </summary>
    public static class ConditionalCases
    {
        /// <summary>A nullable first branch that is null gives null, not the second branch.</summary>
        public static void NullableFirstBranch(FakeWorld world)
        {
            int? none = NoInt();
            int? three = Three();
            Expect.Equal(true, Pick(true, none, 5) == null, "a null int? with a true condition");
            Expect.Equal(3, Pick(true, three, 5) ?? -1, "an int? 3");
            Expect.Equal(5, Pick(false, three, 5) ?? -1, "a false condition");
            Expect.Equal(true, PickLater(true, none, 5) == null, "a null int? through a call");
        }

        /// <summary>A bool? that is false gives false, and a null one null.</summary>
        public static void NullableBoolFirstBranch(FakeWorld world)
        {
            bool? no = No();
            bool? none = NoBool();
            Expect.Equal(false, PickBool(true, no, true) ?? true, "a bool? false");
            Expect.Equal(true, PickBool(true, none, false) == null, "a null bool?");
            Expect.Equal(true, PickBool(false, no, true) ?? false, "a false condition");
        }

        /// <summary>A struct type parameter's value that is false gives false.</summary>
        public static void StructTypeParameter(FakeWorld world)
        {
            Expect.Equal(false, PickValue(true, false, true), "false as a type parameter's value");
            Expect.Equal(7, PickValue(true, 7, 8), "7 as a type parameter's value");
            Expect.Equal(8, PickValue(false, 7, 8), "a false condition");
        }

        /// <summary>A plain value, a reference and a condition that checks for null still give the right branch.</summary>
        public static void StillRight(FakeWorld world)
        {
            int? three = Three();
            int? none = NoInt();
            Expect.Equal(4, PlainPick(true, 4, 5), "a plain int");
            Expect.Equal(5, PlainPick(false, 4, 5), "a plain int, false condition");
            Expect.Equal(3, (three != null ? three : 9) ?? -1, "a null check on the first branch");
            Expect.Equal(9, (none != null ? none : 9) ?? -1, "a null check on a null first branch");
            Expect.Equal("a", Name(true, "a", "b"), "a string");
            Expect.Equal(true, Name(true, null, "b") == null, "a null string");
        }

        private static int? Pick(bool condition, int? first, int? second)
        {
            return condition ? first : second;
        }

        private static int? PickLater(bool condition, int? first, int second)
        {
            return condition ? Same(first) : second;
        }

        private static int? Same(int? value)
        {
            return value;
        }

        private static bool? PickBool(bool condition, bool? first, bool? second)
        {
            return condition ? first : second;
        }

        private static T PickValue<T>(bool condition, T first, T second) where T : struct
        {
            return condition ? first : second;
        }

        private static int PlainPick(bool condition, int first, int second)
        {
            return condition ? first : second;
        }

        private static string? Name(bool condition, string? first, string? second)
        {
            return condition ? first : second;
        }

        private static int? NoInt()
        {
            return null;
        }

        private static int? Three()
        {
            return 3;
        }

        private static bool? No()
        {
            return false;
        }

        private static bool? NoBool()
        {
            return null;
        }
    }
}
