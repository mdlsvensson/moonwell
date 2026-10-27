namespace Moonwell.Testing
{
    /// <summary>
    /// Cases for whole-number floats (CSharp.lua problem C18): the build writes them so they reach Lua as floats, not integers.
    /// The Lua host runs 64-bit integers and doubles where the game runs 32-bit integers and single floats, so an integer
    /// product that wraps in the game does not wrap here, and 16777216f + 1f is 16777217 here as a float: the cases check
    /// the number's Lua type, which is what decides both in the game.
    /// </summary>
    public static class FloatCases
    {
        private const float Four = 4f;

        /// <summary>A whole-number float literal, and arithmetic on it, is a float.</summary>
        public static void WholeLiteral(FakeWorld world)
        {
            float a = 16777216f;
            Expect.Equal("float", NumberType(a), "16777216f");
            Expect.Equal("float", NumberType(a + 1f), "16777216f + 1f in a variable");
        }

        /// <summary>Products of whole-number floats past 2^31 stay floats, so the game does not wrap them.</summary>
        public static void NoWrap(FakeWorld world)
        {
            float big = 50000f;
            Expect.Equal("float", NumberType(big * big), "50000f * 50000f");
            Expect.True(big * big > 2.49e9f && big * big < 2.51e9f, "50000f * 50000f is 2.5e9");
            float r = 600f;
            Expect.Equal("float", NumberType(r * r * r * 10f), "600f * 600f * 600f * 10f");
            Expect.True(r * r * r * 10f > 2.15e9f && r * r * r * 10f < 2.17e9f, "600f * 600f * 600f * 10f is 2.16e9");
        }

        /// <summary>Constants CSharp.lua folds or inlines by value are floats too.</summary>
        public static void Folded(FakeWorld world)
        {
            Expect.Equal("float", NumberType(16777216f + 1f), "(16777216f + 1f), folded");
            Expect.Equal("float", NumberType(Four), "a const float");
            Expect.Equal("float", NumberType(Unit.DefaultFacing), "Moonwell's const float");
            Expect.Equal("float", NumberType((float)7), "(float)7");
            Expect.Equal("float", NumberType(-5f), "-5f");
            Expect.Equal("float", NumberType(true ? 2f : 3f), "a constant conditional");
            float fromInt = 2;
            Expect.Equal("float", NumberType(fromInt), "an int constant stored in a float");
            double d = 5d;
            Expect.Equal("float", NumberType(d), "5d");
            double fromFloat = 2f;
            Expect.Equal("float", NumberType(fromFloat), "a float constant stored in a double");
        }

        /// <summary>Fields' initializers, compound assignments and default parameter values give floats.</summary>
        public static void Stored(FakeWorld world)
        {
            Expect.Equal("float", NumberType(new Holder().Value), "a float field's initializer");
            float x = 0f;
            Expect.Equal("float", NumberType(x), "0f");
            x += 16777216f;
            x *= 2;
            Expect.Equal("float", NumberType(x), "x += 16777216f; x *= 2");
            Expect.Equal("float", NumberType(Facing()), "a default parameter value");
            Expect.Equal("float", NumberType(Facing(90f)), "an argument");
        }

        /// <summary>A unit's most life and most mana are whole numbers kept as floats: set, then read once the unit is gone.</summary>
        public static void UnitWholeValues(FakeWorld world)
        {
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            unit.MaxLife = 420.7f;
            unit.MaxMana = 99.5f;
            unit.Remove();
            Expect.Near(420.0f, unit.MaxLife, "the most life kept");
            Expect.Equal("float", NumberType(unit.MaxLife), "the most life kept");
            Expect.Near(99.0f, unit.MaxMana, "the most mana kept");
            Expect.Equal("float", NumberType(unit.MaxMana), "the most mana kept");
        }

        /// <summary>An int made a float at run time is a float, as each line run 4 saw in the game as an integer.</summary>
        public static void Converted(FakeWorld world)
        {
            int count = 7;
            Expect.Equal("float", NumberType((float)count), "(float)count");
            Expect.Equal("float", NumberType((double)count), "(double)count");
            float d = count * 1000000;
            Expect.Equal("float", NumberType(d), "float d = count * 1000000");
            int many = 5000;
            float e = many * 1000000;
            Expect.Equal("float", NumberType(e), "float e = many * 1000000");
            int n = 50000;
            Expect.Equal("float", NumberType((float)n * (float)n), "(float)n * (float)n");
            Expect.True((float)n * (float)n > 2.49e9f && (float)n * (float)n < 2.51e9f, "(float)n * (float)n is 2.5e9");
            Expect.Equal("float", NumberType(Facing(count)), "an int argument to a float parameter");
            Expect.Equal("float", NumberType(Back(count)), "an int returned as a float");
            float x = 0.5f;
            x = count;
            Expect.Equal("float", NumberType(x), "an int assigned to a float");
            Expect.Equal("float", NumberType(count > 0 ? count : 2.5f), "an int chosen as a float");
        }

        /// <summary>A float or double field or auto-property without an initializer starts as the float 0.0.</summary>
        public static void Bare(FakeWorld world)
        {
            var bare = new BareHolder();
            Expect.Equal("float", NumberType(bare.Value), "a float field without an initializer");
            Expect.True(bare.Value == 0f, "a float field without an initializer is 0");
            Expect.Equal("float", NumberType(BareHolder.Shared), "a static double field without an initializer");
            Expect.Equal("float", NumberType(bare.Auto), "a float auto-property without an initializer");
        }

        /// <summary>
        /// What Moonwell's own rewrite of the map's code did not reach: an int? made a float?, ints taken as floats in a foreach, a
        /// struct's float field, a float array's items, Math.Floor and Math.Ceiling, and a whole float as text.
        /// </summary>
        public static void Beyond(FakeWorld world)
        {
            int? some = Some();
            float? made = some;
            Expect.Equal("float", NumberType(made ?? 0f), "an int? made a float?");
            Expect.Equal(true, made == 4f, "its value");
            double? madeDouble = some;
            Expect.Equal("float", NumberType(madeDouble ?? 0d), "an int? made a double?");
            foreach (float item in new[] { 3, 4 })
            {
                Expect.Equal("float", NumberType(item), "an int taken as a float in a foreach");
            }

            var spot = new FloatSpot();
            Expect.Equal("float", NumberType(spot.X), "a struct's float field");
            var items = new float[2];
            Expect.Equal("float", NumberType(items[1]), "a float array's item");
            Expect.Equal("float", NumberType((float)System.Math.Floor(2.5f)), "Math.Floor");
            Expect.Equal("float", NumberType((float)System.Math.Ceiling(2.5f)), "Math.Ceiling");
            Expect.Equal("float", NumberType(System.Math.Floor(2.5)), "Math.Floor of a double");
            float defaulted = Defaulted();
            Expect.Equal("float", NumberType(defaulted), "default(float?).GetValueOrDefault()");
            Expect.Equal("4", 4f.ToString(), "a whole float's text");
            Expect.Equal("x4", "x" + 4f, "a whole float joined to a string");
            Expect.Equal("[4|2.5]", $"[{4f}|{2.5f}]", "whole and fractional floats interpolated");
        }

        /// <summary>Ints stay integers.</summary>
        public static void IntsStay(FakeWorld world)
        {
            var seven = 7;
            Expect.Equal(3, seven / 2, "7 / 2");
            Expect.Equal("integer", NumberType(7), "7");
            Expect.Equal("integer", NumberType(seven / 2), "7 / 2");
            Expect.Equal("integer", NumberType(int.MaxValue), "int.MaxValue");
        }

        /// <summary>An int field, auto-property, enum field, struct field or default(int) starts as the integer 0, and a loop counts in integers.</summary>
        public static void IntsStart(FakeWorld world)
        {
            var bare = new IntHolder();
            Expect.Equal("integer", NumberType(bare.Count), "an int field without an initializer");
            Expect.Equal("integer", NumberType(IntHolder.Shared), "a static int field without an initializer");
            Expect.Equal("integer", NumberType(bare.Auto), "an int auto-property without an initializer");
            Expect.Equal("integer", NumberType((int)bare.Pick), "an enum field without an initializer");
            Expect.Equal("integer", NumberType(new IntSpot().N), "a struct's int field");
            int none = default;
            Expect.Equal("integer", NumberType(none), "default(int)");
            Expect.Equal("n=0", "n=" + bare.Count, "an int field joined to a string");
            bare.Count++;
            Expect.Equal("n=1", "n=" + bare.Count, "an int field after ++");
            for (int i = 2; i >= 0; i--)
            {
                Expect.Equal("integer", NumberType(i), "a loop counting down");
            }

            for (int j = 0; j <= 4; j += 2)
            {
                Expect.Equal("integer", NumberType(j), "a loop counting by 2");
            }
        }

        /// <summary>An int given to a user-defined operator's float, or returned by a lambda as a float, is a float.</summary>
        public static void MadeFloatsInCalls(FakeWorld world)
        {
            int count = 3;
            var scale = new Scale();
            Expect.Equal("float", NumberType((scale * count).By), "an int given to an operator's float");
            Expect.Equal("float", NumberType((scale * 2).By), "an int constant given to an operator's float");
            System.Func<float> later = () => count;
            Expect.Equal("float", NumberType(later()), "an int a lambda returns as a float");
        }

        private static float Facing(float facing = 270f)
        {
            return facing;
        }

        private static float Back(int count)
        {
            return count;
        }

        /// @CSharpLua.Template = "math.type({0})"
        private static string NumberType(float value)
        {
            return "float";
        }

        /// @CSharpLua.Template = "math.type({0})"
        private static string NumberType(double value)
        {
            return "float";
        }

        /// @CSharpLua.Template = "math.type({0})"
        private static string NumberType(int value)
        {
            return "integer";
        }

        private static int? Some()
        {
            return 4;
        }

        private static float Defaulted()
        {
            float? none = null;
            return none.GetValueOrDefault();
        }

#pragma warning disable CS0649
        private struct FloatSpot
        {
            public float X;
        }
#pragma warning restore CS0649

        private sealed class Holder
        {
            public float Value = 3f;
        }

        // Never set: the cases read the value the field starts with.
#pragma warning disable CS0649
        private sealed class BareHolder
        {
            public static double Shared;

            public float Value;

            public float Auto { get; set; }
        }

        private enum Shade
        {
            Dark,
            Light,
        }

        private sealed class IntHolder
        {
            public static int Shared;

            public int Count;

            public Shade Pick;

            public int Auto { get; set; }
        }

        private struct IntSpot
        {
            public int N;
        }
#pragma warning restore CS0649

        private struct Scale
        {
            public float By;

            public static Scale operator *(Scale scale, float by)
            {
                scale.By = by;
                return scale;
            }
        }
    }
}
