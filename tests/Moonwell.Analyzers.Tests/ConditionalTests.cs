namespace Moonwell.Analyzers.Tests;

/// <summary>
/// MW0016: CSharp.lua writes c ? a : b as c and a or b when a's type is a value type, so a first branch that is nil or
/// false in the game gives b (C24).
/// </summary>
public class ConditionalTests
{
    private const string Members = """
            enum Mood { Calm, Angry }
            struct Pair { public int A; }
            static int? field;
            static int? Maybe() => null;
        """;

    private const string Generic = """
            static int? Body(bool c, int? n) => c ? n : 4;
            static T Pick<T>(bool c, T t, T u) where T : struct => c ? t : u;
            static T? PickMaybe<T>(bool c, T? t, T u) where T : struct => c ? t : u;
            static T Any<T>(bool c, T t, T u) => c ? t : u;
        """;

    [Fact]
    public void a_conditional_whose_first_branch_can_be_nil_or_false_is_rejected()
    {
        // In the game each gives its second branch when the first is nil (or false): c and n or x.
        Assert.Equal(new[]
        {
            "MW0016: c ? n : x",
            "MW0016: c ? n : m",
            "MW0016: c ? (int?)null : 2",
            "MW0016: c ? default : n",
            "MW0016: c ? default(int?) : n",
            "MW0016: c ? flag : true",
            "MW0016: c ? (bool?)b : null",
            "MW0016: c ? (bool?)false : true",
            "MW0016: c ? flag : null",
            "MW0016: c ? amount : 1f",
            "MW0016: c ? pair : new Pair()",
            "MW0016: c ? mood : Mood.Angry",
            "MW0016: c ? Maybe() : 1",
            "MW0016: c ? field : 1",
            "MW0016: c ? n + 1 : 0",
            "MW0016: c ? n : (object)1",
            "MW0016: c ? (n ?? Maybe()) : 1",
            "MW0016: c ? n : 1.5f",
            "MW0016: c ? n : throw new InvalidOperationException()",
            "MW0016: flag != null ? flag : true",
            "MW0016: c ? (d ? 1 : n) : 2",
            "MW0016: c ? (d ? n : 1) : 2",
            "MW0016: d ? n : 1",
            "MW0016: c ? n : default(int)",
            "MW0016: e ? n : 3",
        }, MapCode.Check("""
            bool c = p.Gold > 1, d = p.Lumber > 1, b = p.Gold > 2;
            int? n = p.Gold > 3 ? 1 : null, m = null;
            int x = 1;
            bool? flag = null;
            float? amount = null;
            Pair? pair = null;
            Mood? mood = null;
            int? a1 = c ? n : x;
            int? a2 = c ? n : m;
            int? a3 = c ? (int?)null : 2;
            int? a4 = c ? default : n;
            int? a5 = c ? default(int?) : n;
            bool? a6 = c ? flag : true;
            bool? a7 = c ? (bool?)b : null;
            bool? a8 = c ? (bool?)false : true;
            bool? a9 = c ? flag : null;
            float? b1 = c ? amount : 1f;
            Pair? b2 = c ? pair : new Pair();
            Mood? b3 = c ? mood : Mood.Angry;
            int? b4 = c ? Maybe() : 1;
            int? b5 = c ? field : 1;
            int? b6 = c ? n + 1 : 0;
            object b7 = c ? n : (object)1;
            int? b8 = c ? (n ?? Maybe()) : 1;
            float? b9 = c ? n : 1.5f;
            int? c1 = c ? n : throw new InvalidOperationException();
            bool? c2 = flag != null ? flag : true;
            int? c3 = c ? (d ? 1 : n) : 2;
            int? c4 = c ? (d ? n : 1) : 2;
            int? c5 = c ? n : default(int);
            Func<bool, int?> c6 = e => e ? n : 3;
            """, Members));
    }

    [Fact]
    public void a_conditional_of_an_expression_bodied_member_or_a_struct_type_parameter_is_rejected()
    {
        // T may be bool, or bool? under T?: c and t or u gives u when t is false. An unconstrained T is written with if.
        Assert.Equal(new[] { "MW0016: c ? n : 4", "MW0016: c ? t : u", "MW0016: c ? t : u" }, MapCode.Check("", Generic));
    }

    [Fact]
    public void a_nullable_first_branch_message_says_why_and_what_to_write()
    {
        Assert.Equal(
            "'c ? n : 2' gives its second branch in the game when its first is null or false, since CSharp.lua writes it with and/or; write an if",
            MapCode.Message("var c = p.Gold > 1; int? n = null; int? r = c ? n : 2;"));
    }

    [Fact]
    public void a_conditional_whose_first_branch_cannot_be_nil_or_false_is_accepted()
    {
        // A reference type, a plain bool and a null first branch are written with if; the others cannot be nil or false
        // when they are chosen, or give nil as C# gives null.
        Assert.Empty(MapCode.Check("""
            bool c = p.Gold > 1, d = p.Lumber > 1, b = p.Gold > 2;
            int? n = p.Gold > 3 ? 1 : null;
            int i = 1, x = 2;
            bool? flag = null;
            string? s = null;
            Pair pair = new Pair(), other = new Pair();
            int? a1 = c ? x : n;
            bool a2 = c ? false : true;
            bool a3 = c ? b : d;
            bool a4 = c ? true : d;
            int? a5 = c ? 1 : null;
            int? a6 = c ? null : (int?)2;
            int? a7 = c ? (int?)5 : null;
            int? a8 = c ? (int?)i : null;
            int? a9 = c ? (int?)(i + 1) : null;
            int? b1 = c ? new int?(i) : null;
            bool? b2 = c ? (bool?)true : null;
            int? b3 = c ? i : n;
            int? b4 = c ? n ?? 3 : null;
            int b5 = c ? n!.Value : 1;
            Pair b6 = c ? pair : other;
            string b7 = c ? s ?? "" : "x";
            string? b8 = c ? s : "x";
            object? b9 = c ? (object)i : null;
            Mood? c1 = c ? Mood.Calm : null;
            int? c2 = n != null ? n : x;
            int? c3 = null != n ? n : x;
            int? c4 = n.HasValue ? n : x;
            int? c5 = n is not null ? n : x;
            int? c6 = n is { } ? n : x;
            int? c7 = n != null && d ? n : x;
            int? c8 = n is int ? n : x;
            int? c9 = n is int v ? n : x;
            int? d1 = c ? n : null;
            int? d2 = c ? n : default;
            int? d3 = c ? n : (int?)null;
            int? d4 = c ? (n ?? Maybe()) : null;
            bool? d5 = c ? null : flag;
            int? d6;
            if (c)
            {
                d6 = n;
            }
            else
            {
                d6 = x;
            }
            """, """
                enum Mood { Calm, Angry }
                struct Pair { public int A; }
                static int? Maybe() => null;
                static T Any<T>(bool c, T t, T u) => c ? t : u;
                static T Reference<T>(bool c, T t, T u) where T : class? => c ? t : u;
            """));
    }
}
