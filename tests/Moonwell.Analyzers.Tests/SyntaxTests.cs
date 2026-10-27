namespace Moonwell.Analyzers.Tests;

/// <summary>
/// MW0017 to MW0034: C# syntax and library calls that CSharp.lua stops on, writes wrong, or the game's Lua core does not have.
/// Each was seen by running the code on .NET and in Lua and comparing what they printed.
/// </summary>
public class SyntaxTests
{
    private const string Types = """
            enum Color { Red, Green = 5, Blue }
            [Flags] enum Mode { None = 0, A = 1, B = 2 }
            struct Pt { public int X; }
            class Box { public int X; public Box? Inner; }
            const int N = -5;
    """;

    [Theory]
    // MW0017: patterns the compiler stops on
    [InlineData("MW0017", "int x = 1; var s = x switch { 1 => \"a\", > 10 => \"b\", _ => \"c\" };", "> 10")]
    [InlineData("MW0017", "object o = 1; var s = o switch { int => \"a\", _ => \"c\" };", "int")]
    [InlineData("MW0017", "int x = 1; var s = x switch { var n when n > 3 => \"a\", _ => \"c\" };", "var n")]
    [InlineData("MW0017", "int x = 1; var s = x switch { not 1 => \"a\", _ => \"c\" };", "not 1")]
    [InlineData("MW0017", "int x = 1; var b = x is not (1 or 2);", "(1 or 2)")]
    [InlineData("MW0017", "int x = 1; var b = x is (> 1 and < 9);", "(> 1 and < 9)")]
    [InlineData("MW0017", "var bx = new Box(); var b = bx is { X: var y };", "var y")]
    [InlineData("MW0017", "var bx = new Box(); var b = bx is { X: int y };", "int y")]
    [InlineData("MW0017", "var bx = new Box(); var b = bx is { Inner: not null };", "not null")]
    [InlineData("MW0017", "int[] a = { 1, 2 }; var b = a is [var first, .. var rest];", ".. var rest")]
    // MW0018: patterns that give the wrong answer
    [InlineData("MW0018", "var bx = new Box(); var b = bx is { Inner.X: 9 };", "Inner.X: 9")]
    [InlineData("MW0018", "(int, int) t = (1, 2); var b = t is (1, _);", "(1, _)")]
    // MW0020: a struct changed through a copy of an element
    [InlineData("MW0020", "var arr = new Pt[1]; var c = arr[0]; c.X = 6;", "arr[0]")]
    [InlineData("MW0020", "var l = new List<Pt> { new Pt() }; var c = l[0]; c.X++;", "l[0]")]
    [InlineData("MW0020", "var d = new Dictionary<int, Pt>(); Pt c; c = d[1]; c.X += 2;", "d[1]")]
    // MW0022: tuples
    [InlineData("MW0022", "var (a, b) = (1, 2);", "var (a, b) = (1, 2)")]
    [InlineData("MW0022", "var x = new[] { 1, 2 }; (x[0], x[1]) = (x[1], x[0]);", "(x[0], x[1]) = (x[1], x[0])")]
    [InlineData("MW0022", "var t = (1, 2); var s = \"t=\" + t;", "t")]
    [InlineData("MW0022", "var t = (1, 2); Console.WriteLine(t);", "t")]
    [InlineData("MW0022", "var t = (1, 2); var s = $\"{t}\";", "{t}")]
    // MW0023: declarations
    [InlineData("MW0023", "var o = new { A = 1 }; var w = o with { A = 2 };", "o with { A = 2 }")]
    // MW0024: ranges
    [InlineData("MW0024", "var a = new[] { 1, 2, 3 }; var b = a[1..2];", "1..2")]
    [InlineData("MW0024", "var l = new List<int> { 1 }; var x = l[^1];", "^1")]
    [InlineData("MW0024", "var s = \"abc\"; var t = s[1..];", "1..")]
    // MW0025: ref
    [InlineData("MW0025", "var a = new[] { 1, 2 }; ref int r = ref a[1];", "ref a[1]")]
    [InlineData("MW0025", "var a = new[] { 1, 2 }; ref int r = ref First(a);", "ref First(a)", "static ref int First(int[] a) => ref a[0];")]
    // MW0026: params
    [InlineData("MW0026", "Count(1, 2);", "params List<int> xs", "static int Count(params List<int> xs) => xs.Count;")]
    [InlineData("MW0026", "var f = (params int[] xs) => xs.Length;", "params int[] xs")]
    // MW0027: strings
    [InlineData("MW0027", "var n = \"a--b\".Split(\"--\").Length;", "\"a--b\".Split(\"--\")")]
    [InlineData("MW0027", "var n = \"a, b\".Split(new[] { \", \" }, StringSplitOptions.None).Length;", "\"a, b\".Split(new[] { \", \" }, StringSplitOptions.None)")]
    [InlineData("MW0027", "var b = \"abc\".StartsWith('a');", "\"abc\".StartsWith('a')")]
    [InlineData("MW0027", "var b = \"abc\".EndsWith('c');", "\"abc\".EndsWith('c')")]
    [InlineData("MW0027", "var i = \"abc\".IndexOf(\"B\", StringComparison.OrdinalIgnoreCase);", "\"abc\".IndexOf(\"B\", StringComparison.OrdinalIgnoreCase)")]
    [InlineData("MW0027", "var b = \"Abc\".StartsWith(\"ab\", StringComparison.OrdinalIgnoreCase);", "\"Abc\".StartsWith(\"ab\", StringComparison.OrdinalIgnoreCase)")]
    [InlineData("MW0027", "var r = new string(\"abc\".Reverse().ToArray());", "\"abc\".Reverse()")]
    // MW0028: char or enum given to an object parameter
    [InlineData("MW0028", "char c = 'a'; var s = string.Format(\"{0}\", c);", "c")]
    [InlineData("MW0028", "var c = Color.Green; Console.WriteLine(c);", "c")]
    [InlineData("MW0028", "var sb = new System.Text.StringBuilder(); sb.Append((object)Color.Green);", "Color.Green")]
    // MW0029: catch
    [InlineData("MW0029", "try { } catch (NullReferenceException) { }", "NullReferenceException")]
    [InlineData("MW0029", "try { } catch (IndexOutOfRangeException) { }", "IndexOutOfRangeException")]
    [InlineData("MW0029", "try { } catch (Exception e) when (e is InvalidOperationException ioe) { }", "InvalidOperationException ioe")]
    // MW0030: members the Lua core does not have
    [InlineData("MW0030", "var q = new[] { 1 }.Append(2);", "Append")]
    [InlineData("MW0030", "var q = new[] { 1 }.TakeLast(1);", "TakeLast")]
    [InlineData("MW0030", "var q = new[] { 1 }.Zip(new[] { 2 });", "Zip")]
    [InlineData("MW0030", "var q = new[] { 1 }.ToHashSet();", "ToHashSet")]
    [InlineData("MW0030", "var q = new List<int>().ConvertAll(x => x);", "ConvertAll")]
    [InlineData("MW0030", "var sb = new System.Text.StringBuilder(\"a\"); sb.Replace(\"a\", \"b\");", "Replace")]
    [InlineData("MW0030", "var d = new Dictionary<int, int>(); var v = d.GetValueOrDefault(1);", "GetValueOrDefault")]
    [InlineData("MW0030", "var d = new Dictionary<int, int>(); d.Remove(1, out var v);", "Remove")]
    [InlineData("MW0030", "var d = new Dictionary<int, int>(); var l = d.ToList();", "ToList")]
    [InlineData("MW0030", "var q = new Queue<int>(); q.TryPeek(out var x);", "TryPeek")]
    [InlineData("MW0030", "var s = new Stack<int>(); s.TryPop(out var x);", "TryPop")]
    [InlineData("MW0030", "var h = new HashSet<int>(); var b = h.SetEquals(new[] { 1 });", "SetEquals")]
    [InlineData("MW0030", "var c = StringComparer.Ordinal;", "StringComparer")]
    [InlineData("MW0030", "var g = Guid.NewGuid();", "Guid")]
    [InlineData("MW0030", "var n = Environment.NewLine;", "NewLine")]
    [InlineData("MW0030", "var k = new[] { 2, 1 }; var v = new[] { \"b\", \"a\" }; Array.Sort(k, v);", "Sort")]
    [InlineData("MW0030", "var q = Math.DivRem(7, 2, out var r);", "DivRem")]
    [InlineData("MW0030", "var n = HashCode.Combine(1, 2);", "HashCode")]
    [InlineData("MW0030", "Index i = 1;", "Index")]
    // MW0031: checked arithmetic
    [InlineData("MW0031", "int x = 1; var y = checked(x + 1);", "checked(x + 1)")]
    [InlineData("MW0031", "int x = 1; checked { x++; }", "checked { x++; }")]
    // MW0032: enum text
    [InlineData("MW0032", "var c = Enum.Parse<Color>(\"Blue\");", "Enum.Parse<Color>(\"Blue\")")]
    [InlineData("MW0032", "var c = Enum.GetValues<Color>();", "Enum.GetValues<Color>()")]
    [InlineData("MW0032", "var s = Color.Green.ToString(\"D\");", "Color.Green.ToString(\"D\")")]
    [InlineData("MW0032", "var m = Mode.A | Mode.B; var s = \"m=\" + m;", "m")]
    [InlineData("MW0032", "var s = (Color)5 + \" x\";", "(Color)5")]
    // MW0034: using order
    [InlineData("MW0034", "using var a = new Res(); using var b = new Res();", "using var b = new Res();", "class Res : IDisposable { public void Dispose() { } }")]
    // MW0005: decimal
    [InlineData("MW0005", "decimal d = 1;", "d")]
    public void rejected(string id, string body, string pointsAt, string members = "")
    {
        var found = MapCode.Check(body, Types + "\n" + members);
        Assert.Contains(id + ": " + pointsAt, found.Where(f => f.StartsWith(id)).Select(f => f.Replace("\r", "")));
    }

    [Fact]
    public void the_supported_forms_are_accepted()
    {
        Assert.Empty(MapCode.Check("""
            int x = 5;
            var a = x switch { 1 => "one", 2 or 3 => "few", _ => "many" };
            object o = x;
            var b = o switch { int i when i > 3 => "big", int i => "int" + i, string s => s, null => "null", _ => "?" };
            var c = o is int n && n > 3;
            var d = x is > 1 and < 9;
            var e = o is not null;
            var f = o is not string;
            var box = new Box { X = 3 };
            var g = box is { X: 3 } && box is { X: > 2 } && box is { Inner: null };
            (int, int) t = (1, 2);
            var h = t switch { (1, 2) => "a", _ => "b" };
            int[] arr = { 1, 2, 3 };
            var i1 = arr[^1];
            var j = arr is [1, 2, 3] && arr is [1, ..] && arr is [.., 3];
            (int q, int r) = (1, 2);
            var (first, second) = Pair();
            """, Types + "\n" + "static (int, int) Pair() => (1, 2);"));
    }

    [Fact]
    public void library_calls_that_work_are_accepted()
    {
        Assert.Empty(MapCode.Check("""
            var s = "a b c";
            var parts = s.Split(' ', StringSplitOptions.RemoveEmptyEntries);
            var more = s.Split(',');
            var has = s.Contains('b') && s.StartsWith("a") && s.EndsWith("c") && s.IndexOf("b", StringComparison.Ordinal) >= 0;
            var eq = s.Equals("A", StringComparison.OrdinalIgnoreCase);
            var sb = new System.Text.StringBuilder().Append('x').Append("y").Append(1).AppendLine().Append(Color.Green.ToString());
            Console.WriteLine('c' + "d");
            Console.WriteLine($"{Color.Green} {'c'}");
            var text = "x" + Color.Red + 'c';
            var list = new List<int> { 3, 1 }; list.Sort(); list.RemoveAll(x => x > 5);
            var dict = new Dictionary<string, int>(); dict["a"] = 1; dict.TryGetValue("a", out var one); dict.Remove("a");
            var q = new Queue<int>(); q.TryDequeue(out var first);
            var linq = list.Where(x => x > 1).Select(x => x * 2).OrderBy(x => x).Concat(list).Distinct().ToList();
            try { } catch (InvalidOperationException) { } catch (ArgumentException e) when (e.Message == "x") { } catch (Exception) { }
            var un = unchecked(1 + 2);
            var big = 5; var cast = checked((byte)big);
            var mode = Mode.A | Mode.B; var has2 = mode.HasFlag(Mode.A) && (mode & Mode.B) != 0;
            var named = Color.Green; var back = (Color)5; var ok = back == named;
            """, Types));
    }

    [Fact]
    public void primary_constructor_parameters_are_allowed_in_initializers_and_base()
    {
        var found = MapCode.CheckFile("""
            class Base(string n) { public string N = n; }
            class Der(string n, int k) : Base(n) { public int K = k; public string M() => n + k; public int Z { get; } = k; }
            class Plain(int x) { public int Get() => x; }
            record Rec(int A) { public int B = A; }
            """);
        Assert.Empty(found);
    }

    [Fact]
    public void a_minus_before_a_negative_constant_is_allowed()
    {
        var found = MapCode.Check("""
            var a = -N;
            var b = unchecked(-int.MinValue);
            var c = -(N);
            """, Types);
        Assert.Empty(found);
    }

    [Fact]
    public void initializers_read_in_one_statement_are_allowed()
    {
        var found = MapCode.Check("""
            var s = new Box { X = 1 }.X + " " + new Box { X = 2 }.X;
            var e = new Box { X = 1 }.Equals(new Box { X = 1 }) + " " + object.Equals(new Box { X = 2 }, new Box { X = 2 });
            """, Types);
        Assert.Empty(found);
    }

    [Fact]
    public void user_defined_increment_and_truth_operators_are_rejected()
    {
        var found = MapCode.CheckFile("""
            struct V
            {
                public int X;
                public static V operator ++(V a) => new V { X = a.X + 1 };
                public static V operator -(V a) => new V { X = -a.X };
                public static bool operator true(V a) => a.X != 0;
                public static bool operator false(V a) => a.X == 0;
            }
            """);
        Assert.Equal(3, found.Count);
        Assert.All(found, f => Assert.StartsWith("MW0023: public static", f));
    }

    [Fact]
    public void a_class_with_two_constructions_of_one_generic_interface_is_rejected()
    {
        var found = MapCode.CheckFile("""
            interface IG<T> { T Get(); }
            class E : IG<int>, IG<string> { int IG<int>.Get() => 1; string IG<string>.Get() => "s"; }
            class F : IG<int> { public int Get() => 1; }
            """);
        Assert.Equal(new[] { "MW0023: E" }, found);
    }

    [Fact]
    public void a_derived_record_and_a_static_abstract_member_are_rejected()
    {
        var found = MapCode.CheckFile("""
            record R(int A);
            record D(int A, int B) : R(A);
            interface IMake<T> { static abstract T Make(); }
            interface IPlain { static int Z() => 1; int V(); }
            """);
        Assert.Equal(new[] { "MW0023: record D(int A, int B) : R(A);", "MW0023: static abstract T Make();" }, found);
    }

    [Fact]
    public void a_variable_that_is_not_changed_may_hold_a_copy_of_an_element()
    {
        Assert.Empty(MapCode.Check("""
            var arr = new Pt[1];
            var c = arr[0];
            var x = c.X;
            arr[0] = new Pt { X = 1 };
            var local = new Pt();
            var copy = local; copy.X = 2;
            """, Types));
    }

    [Fact]
    public void two_initializers_beside_each_other_are_accepted()
    {
        Assert.Empty(MapCode.Check("""
            var s = new Box { X = 1 }.Equals(new Box { X = 1 });
            var t = Take(new Box { X = 1 }, new Box { X = 2 });
            """, Types + "\nstatic int Take(Box a, Box b) => a.X + b.X;"));
    }

    [Fact]
    public void the_message_says_what_to_write_instead()
    {
        Assert.Equal(
            "Split(string) runs out of memory in the game's Lua; split on a char",
            MapCode.Message("var n = \"a--b\".Split(\"--\").Length;").Replace("'\"a--b\".Split(\"--\")' ", string.Empty));
    }

    private const string Structs = """
            struct In { public int X; }
            struct Pt2
            {
                public int X;
                public In Inner;
                public void Move() { X++; }
                public void Shift() { Move(); }
                public int Read() => X;
                public readonly int Twice() => X * 2;
            }
            class Holder { public int X; }
            struct WithRef { public Holder H; }
            static void Bump(ref Pt2 p) { p.X++; }
    """;

    [Fact]
    public void a_struct_element_changed_through_a_method_a_nested_field_or_ref_is_mw0020()
    {
        var found = MapCode.Check("""
            var arr = new Pt2[2];
            var f = arr[1];
            f.Move();
            var g = arr[0];
            g.Inner.X = 4;
            var h = arr[0];
            Bump(ref h);
            var list = new List<Pt2> { new Pt2() };
            var k = list[0];
            k.Inner.X++;
            var s = arr[1];
            s.Shift();
            """, Structs);

        Assert.Equal(new[] { "MW0020: arr[1]", "MW0020: arr[0]", "MW0020: arr[0]", "MW0020: list[0]", "MW0020: arr[1]" }, found);
    }

    [Fact]
    public void reading_a_struct_element_copy_or_changing_what_it_points_at_is_accepted()
    {
        Assert.Empty(MapCode.Check("""
            var arr = new Pt2[2];
            var a = arr[0];
            var n = a.Read() + a.Twice() + a.X + a.Inner.X;
            var refs = new WithRef[1];
            var r = refs[0];
            r.H.X = 3;
            """, Structs));
    }

    [Fact]
    public void a_struct_element_copy_written_back_is_accepted()
    {
        Assert.Empty(MapCode.Check("""
            var arr = new Pt2[2];
            var c = arr[0];
            c.X = 6;
            c.Move();
            arr[0] = c;
            var list = new List<Pt2> { new Pt2() };
            var i = 0;
            var d = list[i];
            d.Inner.X = 1;
            list[i] = d;
            """, Structs));
    }

    [Fact]
    public void a_struct_element_read_or_moved_before_the_write_back_is_mw0020()
    {
        var found = MapCode.Check("""
            var arr = new Pt2[2];
            var c = arr[0];
            c.X = 6;
            var seen = arr[0].X;
            arr[0] = c;
            var i = 0;
            var d = arr[i];
            d.X = 1;
            i++;
            arr[i] = d;
            """, Structs);

        Assert.Equal(new[] { "MW0020: arr[0]", "MW0020: arr[i]" }, found);
    }
}
