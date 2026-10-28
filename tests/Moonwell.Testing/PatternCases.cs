namespace Moonwell.Testing
{
    /// <summary>
    /// Cases for the patterns MW0017 and MW0018 allow since decision 80, which the fork's CSharp.lua 2.0.4-moonwell.12
    /// writes right: parentheses in an is test, not inside a property pattern, an extended property pattern
    /// ({ Inner.X: 9 }), and a positional pattern in an is test on a value that cannot be null (a tuple or a struct).
    /// </summary>
    public static class PatternCases
    {
        /// <summary>Parentheses in an is test: at the top, under not, inside a property pattern and around a positional pattern.</summary>
        public static void IsParentheses(FakeWorld world)
        {
            var x = 5;
            Expect.True(x is (1 or 5), "x is (1 or 5)");
            Expect.True(x is (> 0 and < 10), "x is (> 0 and < 10)");
            Expect.True(x is not (1 or 2), "x is not (1 or 2)");
            Expect.True(!(x is (> 6 or < 0)), "x is (> 6 or < 0)");
            Expect.True(x is (> 0 and (< 3 or > 4)), "nested parentheses");
            var node = new Node { X = 3 };
            Expect.True(node is { X: (1 or 3) }, "inside a property pattern");
            Expect.True(!(node is { X: (> 5 and < 9) }), "inside a property pattern, false");
            var pair = (4, 2);
            Expect.True(pair is ((> 3 and < 5), 2), "inside a positional pattern");
        }

        /// <summary>not inside a property pattern, in an is test and in a switch expression arm, on a null and a set member and on a null input.</summary>
        public static void NotInProperty(FakeWorld world)
        {
            var node = new Node { X = 3 };
            Expect.True(!(node is { Name: not null }), "a null name is not not null");
            node.Name = "a";
            Expect.True(node is { Name: not null }, "a set name");
            Expect.True(!(node is { X: not 3 }) && node is { X: not 4 }, "not a constant");
            Expect.True(node is { X: not (1 or 2), Name: not null }, "two members");
            Expect.True(!(node is { Inner: not null }), "a null inner");
            node.Inner = new Node { X = 1 };
            Expect.True(node is { Inner: not null } && !(node is { Inner: not { X: 1 } }), "a set inner");
            Node? none = null;
            Expect.True(!(none is { Name: not null }), "a null input");
            Expect.Equal("named", Named(node), "an arm, a set name");
            Expect.Equal("anon", Named(new Node()), "an arm, a null name");
            Expect.Equal("anon", Named(null), "an arm, a null input");
        }

        /// <summary>An extended property pattern ({ Inner.X: 9 }), in an is test and in a switch expression arm, with a null inner and a null input.</summary>
        public static void ExtendedProperty(FakeWorld world)
        {
            var node = new Node { X = 1, Inner = new Node { X = 9, Inner = new Node { X = 2 } } };
            Expect.True(node is { Inner.X: 9 }, "Inner.X: 9");
            Expect.True(!(node is { Inner.X: 8 }), "Inner.X: 8");
            Expect.True(node is { Inner.Inner.X: 2 }, "two levels");
            Expect.True(node is { Inner.X: > 5 } && !(node is { Inner.X: > 9 }), "relational");
            Expect.True(node is { X: 1, Inner.X: 9 }, "beside a plain member");
            Expect.True(!(node is { Inner.Name: not null }), "with not, a null name");
            Expect.True(!(new Node() is { Inner.X: 9 }), "a null inner");
            Node? none = null;
            Expect.True(!(none is { Inner.X: 9 }), "a null input");
            Expect.Equal("nine", Nine(node), "an arm");
            Expect.Equal("other", Nine(new Node { Inner = new Node { X = 1 } }), "an arm, another value");
            Expect.Equal("other", Nine(new Node()), "an arm, a null inner");
            Expect.Equal("other", Nine(null), "an arm, a null input");
        }

        /// <summary>A positional pattern in an is test on a tuple, a struct with Deconstruct and a record struct: constants, discards, relational, names, not, a property pattern inside, and with and, or and not around it.</summary>
        public static void PositionalOnValue(FakeWorld world)
        {
            var pair = (1, 2);
            Expect.True(pair is (1, _) && !(pair is (2, _)), "a tuple, a constant and a discard");
            Expect.True(pair is (> 0, >= 2) && !(pair is (> 0, > 5)), "a tuple, relational");
            Expect.Equal(12, pair is (var a, var b) ? a * 10 + b : -1, "a tuple, names");
            Expect.True(pair is (not 3, _), "a tuple, not inside");
            Expect.True(pair is (2, _) or (1, _), "or around it");
            Expect.True(pair is (1, _) and (_, 2), "and around it");
            Expect.True(!(pair is not (1, _)), "not around it");
            var withNode = (new Node { Name = "a" }, 1);
            Expect.True(withNode is ({ Name: not null }, 1), "a property pattern inside");
            var size = new Size { Width = 1, Height = 2 };
            Expect.True(size is (1, 2) && !(size is (1, 3)), "a struct");
            Expect.Equal(2, size is Size(1, var h) ? h : -1, "a struct named, with a name inside");
            var spot = new Spot(1, 2);
            Expect.True(spot is (1, 2) && !(spot is (1, 3)), "a record struct");
            Expect.Equal(2, spot is Spot(_, var y) ? y : -1, "a record struct named");
        }

        /// <summary>The name of a node's kind: whether it has a name.</summary>
        private static string Named(Node? node) => node switch { { Name: not null } => "named", _ => "anon" };

        /// <summary>Whether a node's inner X is 9.</summary>
        private static string Nine(Node? node) => node switch { { Inner.X: 9 } => "nine", _ => "other" };

        /// <summary>A class with a number, a name and an inner node.</summary>
        private sealed class Node
        {
            public int X;

            public string? Name;

            public Node? Inner;
        }

        /// <summary>A struct that deconstructs into its width and height.</summary>
        private struct Size
        {
            public int Width;

            public int Height;

            /// <summary>Gives the width and height.</summary>
            public void Deconstruct(out int width, out int height)
            {
                width = Width;
                height = Height;
            }
        }

        /// <summary>A record struct of two numbers.</summary>
        private record struct Spot(int X, int Y);
    }
}
