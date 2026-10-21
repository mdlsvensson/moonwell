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

        /// <summary>Ints stay integers.</summary>
        public static void IntsStay(FakeWorld world)
        {
            var seven = 7;
            Expect.Equal(3, seven / 2, "7 / 2");
            Expect.Equal("integer", NumberType(7), "7");
            Expect.Equal("integer", NumberType(seven / 2), "7 / 2");
            Expect.Equal("integer", NumberType(int.MaxValue), "int.MaxValue");
        }

        private static float Facing(float facing = 270f)
        {
            return facing;
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

        private sealed class Holder
        {
            public float Value = 3f;
        }
    }
}
