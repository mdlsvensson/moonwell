namespace Moonwell.Testing
{
    /// <summary>
    /// Cases for values made text: a nullable value or a number in a string, in interpolation and through ToString, gives
    /// what C# gives (a null nullable gives ""). The game's Lua has no debug library, so numbers and booleans have no
    /// methods there (CSharp.lua problem C21); the Lua host runs the script without it too.
    /// </summary>
    public static class TextCases
    {
        /// <summary>A nullable int, float or bool joined to a string gives its text, and "" when null.</summary>
        public static void NullableConcat(FakeWorld world)
        {
            int? three = Three();
            int? noInt = NoInt();
            float? half = Half();
            float? noFloat = NoFloat();
            bool? yes = Yes();
            bool? noBool = NoBool();
            Expect.Equal("n=3", "n=" + three, "an int? 3");
            Expect.Equal("n=", "n=" + noInt, "a null int?");
            Expect.Equal("f=2.5", "f=" + half, "a float? 2.5");
            Expect.Equal("f=", "f=" + noFloat, "a null float?");
            Expect.Equal("b=True", "b=" + yes, "a bool? true");
            Expect.Equal("b=", "b=" + noBool, "a null bool?");
            Expect.Equal("3!", three + "!", "an int? before the string");
            Expect.Equal("!", noInt + "!", "a null int? before the string");
        }

        /// <summary>A nullable value read through ?. joined to a string, as the map's count of a missing object.</summary>
        public static void ConditionalAccessConcat(FakeWorld world)
        {
            var some = new Bag();
            Bag? none = NoBag();
            Expect.Equal("count 3", "count " + some?.Count, "a count read through ?.");
            Expect.Equal("count ", "count " + none?.Count, "a count of a null bag through ?.");
            Expect.Equal("on True", "on " + some?.On, "a bool read through ?.");
            Expect.Equal("on ", "on " + none?.On, "a bool of a null bag through ?.");
        }

        /// <summary>A nullable value interpolated gives its text, and "" when null.</summary>
        public static void NullableInterpolation(FakeWorld world)
        {
            int? three = Three();
            int? noInt = NoInt();
            float? half = Half();
            float? noFloat = NoFloat();
            bool? yes = Yes();
            bool? noBool = NoBool();
            Expect.Equal("[3]", $"[{three}]", "an int? 3");
            Expect.Equal("[]", $"[{noInt}]", "a null int?");
            Expect.Equal("[2.5]", $"[{half}]", "a float? 2.5");
            Expect.Equal("[]", $"[{noFloat}]", "a null float?");
            Expect.Equal("[True]", $"[{yes}]", "a bool? true");
            Expect.Equal("[]", $"[{noBool}]", "a null bool?");
            Expect.Equal("[2.5|003]", $"[{half:F1}|{three:D3}]", "a float? and an int? with a format");
            Expect.Equal("[|]", $"[{noFloat:F1}|{noInt:D3}]", "a null float? and int? with a format");
            Expect.Equal("[True|]", $"[{yes:G}|{noBool:G}]", "a bool? with a format, which C# leaves out");
            Expect.Equal("[  3|   ]", $"[{three,3}|{noInt,3}]", "an int? with a width");
            Expect.Equal("[ 2.5]", $"[{half,4:F1}]", "a float? with a width and a format");
            var none = NoBag();
            Expect.Equal("[]", $"[{none?.Count}]", "a count of a null bag through ?.");
        }

        /// <summary>A nullable value worked out in the expression, added with +=, or made from a cast, joined to a string.</summary>
        public static void NullableOperands(FakeWorld world)
        {
            int? three = Three();
            int? noInt = NoInt();
            int plain = 4;
            var text = "s=";
            text += three;
            text += noInt;
            Expect.Equal("s=3", text, "+= an int?");
            Expect.Equal("x4", "x" + (three + 1), "an int? sum");
            Expect.Equal("x", "x" + (noInt + 1), "a null int? sum");
            Expect.Equal("x4", "x" + (int?)plain, "an int cast to an int?");
            Expect.Equal("x3", "x" + (plain > 0 ? three : null), "a conditional int?");
            Expect.Equal("x", "x" + (plain < 0 ? three : null), "a conditional null int?");
            Expect.Equal("123", 'x' + three + "", "a char added to an int?");
            Expect.Equal("x3 y", "x" + three + " y" + noInt, "two int? in one string");
        }

        /// <summary>n?.ToString() gives the text, and null when n is null.</summary>
        public static void NullableToString(FakeWorld world)
        {
            int? three = Three();
            int? noInt = NoInt();
            float? half = Half();
            bool? yes = Yes();
            bool? noBool = NoBool();
            Expect.Equal("3", three?.ToString(), "int? 3 ?.ToString()");
            Expect.Equal(null, noInt?.ToString(), "a null int? ?.ToString()");
            Expect.Equal("2.5", half?.ToString(), "float? 2.5 ?.ToString()");
            Expect.Equal("True", yes?.ToString(), "bool? true ?.ToString()");
            Expect.Equal(null, noBool?.ToString(), "a null bool? ?.ToString()");
            Expect.Equal("3", three.ToString(), "int? 3 .ToString()");
            Expect.Equal("", noInt.ToString(), "a null int? .ToString()");
            Expect.Equal("x", "x" + noInt?.ToString(), "a null int? ?.ToString() joined");
        }

        /// <summary>A plain int, float or bool, and one boxed as an object, gives the text C# gives.</summary>
        public static void Plain(FakeWorld world)
        {
            int three = 3;
            float half = 2.5f;
            bool yes = true;
            Expect.Equal("3", three.ToString(), "3.ToString()");
            Expect.Equal("2.5", half.ToString(), "2.5f.ToString()");
            Expect.Equal("True", yes.ToString(), "true.ToString()");
            Expect.Equal("n=3 f=2.5 b=True", "n=" + three + " f=" + half + " b=" + yes, "joined");
            Expect.Equal("n=3 f=2.5 b=True", $"n={three} f={half} b={yes}", "interpolated");
            object boxed = three;
            object boxedBool = yes;
            Expect.Equal("3", boxed.ToString(), "a boxed int's ToString()");
            Expect.Equal("True", boxedBool.ToString(), "a boxed bool's ToString()");
            Expect.Equal("o=3", "o=" + boxed, "a boxed int joined");
            Expect.Equal("o=True", "o=" + boxedBool, "a boxed bool joined");
            object? boxedNull = NoInt();
            Expect.Equal("o=", "o=" + boxedNull, "a boxed null int? joined");
        }

        private static int? Three()
        {
            return 3;
        }

        private static int? NoInt()
        {
            return null;
        }

        private static float? Half()
        {
            return 2.5f;
        }

        private static float? NoFloat()
        {
            return null;
        }

        private static bool? Yes()
        {
            return true;
        }

        private static bool? NoBool()
        {
            return null;
        }

        private static Bag? NoBag()
        {
            return null;
        }

        private sealed class Bag
        {
            public int Count = 3;

            public bool On = true;
        }
    }
}
