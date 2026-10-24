using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>
    /// Cases that pin what the maths design rests on in Lua: a readonly struct is shared, never copied, and nothing can change
    /// it (CSharp.lua leaves out the copy of a mutable struct added to a List or passed to a lambda, C25 and C26); two
    /// results come back through out with no table; and a vector's components are Lua floats.
    /// </summary>
    public static class LuaShapeCases
    {
        private sealed class Holder
        {
            public Vector2 Field;
        }

        /// <summary>A vector stored in a field, a List and passed to a lambda reads the same values afterwards.</summary>
        public static void SharedNotCopied(FakeWorld world)
        {
            var v = new Vector2(3f, 4f);
            var holder = new Holder { Field = v };
            var list = new List<Vector2> { v };
            Func<Vector2, float> read = given => given.X + given.Y;
            Expect.Near(7f, read(v), "the lambda read it");
            v = v + Vector2.One;
            Expect.True(holder.Field == new Vector2(3f, 4f), "the field kept (3, 4)");
            Expect.True(list[0] == new Vector2(3f, 4f), "the list kept (3, 4)");
            Expect.True(v == new Vector2(4f, 5f), "the local is a new vector");
        }

        /// <summary>Maths.Polar gives both coordinates through out.</summary>
        public static void OutPair(FakeWorld world)
        {
            Maths.Polar(1f, 2f, 0f, 3f, out var x, out var y);
            Expect.Near(4f, x, "x");
            Expect.Near(2f, y, "y");
        }

        /// <summary>A vector's components are Lua floats, made from whole numbers and computed.</summary>
        public static void NumberTypes(FakeWorld world)
        {
            var made = new Vector2(1f, 2f);
            var computed = made * 2f + Vector2.One;
            Expect.Equal("float", NumberType(made.X), "made X");
            Expect.Equal("float", NumberType(made.Y), "made Y");
            Expect.Equal("float", NumberType(computed.X), "computed X");
            Expect.Equal("float", NumberType(Vector2.Zero.Y), "Zero.Y");
            Expect.Equal("float", NumberType(new Vector3(1f, 2f, 3f).Z), "Vector3 Z");
            Expect.Equal("float", NumberType(Angles.Normalize(-90f)), "a normalized angle");
        }

        /// @CSharpLua.Template = "math.type({0})"
        private static string NumberType(float value)
        {
            return "float";
        }
    }
}
