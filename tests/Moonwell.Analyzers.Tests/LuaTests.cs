namespace Moonwell.Analyzers.Tests;

/// <summary>MW0007 and MW0008: C# that CSharp.lua cannot compile.</summary>
public class LuaTests
{
    [Fact]
    public void collection_expressions_are_rejected_wherever_they_are()
    {
        Assert.Equal(new[]
        {
            "MW0007: [1, 2]",
            "MW0007: []",
            "MW0007: [..a, 3]",
            "MW0007: [4]",
            "MW0007: [5]",
        }, MapCode.Check("""
            List<int> a = [1, 2];
            a = [];
            var b = new List<int>([..a, 3]);
            Take([4]);
            """, """
                static int[] Field = [5];
                static void Take(List<int> list) { }
            """));
    }

    [Fact]
    public void a_collection_expression_message_gives_what_to_write()
    {
        Assert.Equal(
            "CSharp.lua drops a collection expression or stops on it; write an initializer such as new List<int> { 1, 2 }, new() { 1, 2 } or new[] { 1, 2 }",
            MapCode.Message("List<int> a = [1];"));
    }

    [Fact]
    public void collection_initializers_and_indexers_are_accepted()
    {
        Assert.Empty(MapCode.Check("""
            var a = new List<int> { 1, 2 };
            List<int> b = new() { 3 };
            var c = new[] { 4 };
            var d = a[0] + c[0];
            """));
    }

    [Fact]
    public void using_static_natives_is_rejected()
    {
        Assert.Equal(new[] { "MW0008: using static Moonwell.Natives;", "MW0008: using static Natives;" },
            MapCode.CheckFile("""
                using static Moonwell.Natives;

                namespace Moonwell.Map
                {
                    using static Natives;

                    public static class Code
                    {
                        public static int Run() => GetRandomInt(1, 2);
                    }
                }
                """));
    }

    [Fact]
    public void using_static_natives_message_gives_what_to_write()
    {
        Assert.Equal(
            "using static Moonwell.Natives makes CSharp.lua stop on a handle constant (such as ATTACK_TYPE_MAGIC) and hides Moonwell.Player behind Natives.Player; write Natives.X",
            Assert.Single(MapCode.MessagesOfFile("""
                using static Moonwell.Natives;

                public static class Code
                {
                }
                """)));
    }

    [Fact]
    public void other_static_imports_and_natives_by_name_are_accepted()
    {
        Assert.Empty(MapCode.CheckFile("""
            using static System.Math;
            using Moonwell;

            public static class Code
            {
                public static float Run() => Max(1f, 2f) + Natives.GetRandomInt(1, 2);
            }
            """));
    }
}
