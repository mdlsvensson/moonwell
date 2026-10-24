using static Moonwell.Testing.TemplateValues;

namespace Moonwell.Testing
{
    /// <summary>
    /// Cases for properties CSharp.lua writes from a template: read through <c>using static</c> (C2) and with no receiver,
    /// through <c>this.</c> or <c>base.</c> (C20), CSharp.lua stopped with a NullReferenceException.
    /// </summary>
    public static class TemplateCases
    {
        /// <summary>A static template property used through <c>using static</c> gives its template's value.</summary>
        public static void StaticThroughUsingStatic(FakeWorld world)
        {
            Expect.Equal(42, Answer, "a static template property by its bare name");
            Expect.Equal(42, TemplateValues.Answer, "the same by the class");
        }

        /// <summary>An instance template property gives its template's value read by its name, through this. and through base.</summary>
        public static void InstanceWithoutReceiver(FakeWorld world)
        {
            var crate = new Crate();
            var big = new BigCrate();
            Expect.Equal(10, crate.ReadBare(), "read by its name");
            Expect.Equal(10, crate.ReadThis(), "read through this.");
            Expect.Equal(10, crate.Twice, "read through the object");
            Expect.Equal(10, big.ReadBare(), "read by its name in a derived class");
            Expect.Equal(10, big.ReadBase(), "read through base.");
        }

        /// <summary>An instance template property put by its name, through this. and in an object initializer.</summary>
        public static void InstancePut(FakeWorld world)
        {
            var crate = new Crate();
            crate.PutBare(7);
            Expect.Equal(7, crate.Size, "put by its name");
            crate.PutThis(8);
            Expect.Equal(8, crate.Size, "put through this.");
            var made = new Crate { Resized = 9 };
            Expect.Equal(9, made.Size, "put in an object initializer");
        }
    }

    /// <summary>Values the Lua writes from a template; the body is what .NET runs, and gives the same.</summary>
    public static class TemplateValues
    {
        /// @CSharpLua.Get = "42"
        public static int Answer { get { return 42; } }
    }

    /// <summary>A class whose properties are written from the field <c>Size</c>.</summary>
    public class Crate
    {
        public int Size = 5;

        /// @CSharpLua.Get = "{this}.Size * 2"
        public int Twice { get { return Size * 2; } }

        /// @CSharpLua.Set = "{this}.Size = {0}"
        public int Resized { set { Size = value; } }

        public int ReadBare()
        {
            return Twice;
        }

        public int ReadThis()
        {
            return this.Twice;
        }

        public void PutBare(int size)
        {
            Resized = size;
        }

        public void PutThis(int size)
        {
            this.Resized = size;
        }
    }

    /// <summary>A crate that reads its base's template property.</summary>
    public class BigCrate : Crate
    {
        public int ReadBase()
        {
            return base.Twice;
        }
    }
}
