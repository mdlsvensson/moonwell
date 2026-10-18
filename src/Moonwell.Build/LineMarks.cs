using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;

namespace Moonwell.Build
{
    /// <summary>
    /// Puts a location marker before every statement of a C# file, which CSharp.lua copies into the script as a line of its
    /// own (<c>--@mw 3:12</c>) just before that statement's Lua; <see cref="LineTable"/> turns them into the script's line table.
    /// </summary>
    public static class LineMarks
    {
        /// <summary>
        /// Inserts <c>/*[[--@mw F:L]]*/ </c> at the start of each statement of a block or a switch section (F the file's number,
        /// L the statement's line), and before each block's <c>}</c> the location of the statement the block is part of, or
        /// <c>/*[[--@mw -]]*/ </c> when it is a member's body: lines after it name no C# line. The markers are inline, so no line
        /// moves; CSharp.lua's code template (<c>/*[[...]]*/</c>) keeps them when comments are off.
        /// </summary>
        /// <param name="source">The file's text.</param>
        /// <param name="file">The file's number in the line table, from 1.</param>
        /// <param name="symbols">The preprocessor symbols CSharp.lua compiles with; statements in inactive branches get no marker.</param>
        /// <returns>The marked text.</returns>
        public static string Mark(string source, int file, IEnumerable<string> symbols)
        {
            var options = CSharpParseOptions.Default.WithLanguageVersion(LanguageVersion.Latest).WithPreprocessorSymbols(symbols);
            var root = CSharpSyntaxTree.ParseText(source, options).GetRoot();
            var inserts = new List<(int Position, string Text)>();
            foreach (var node in root.DescendantNodes())
            {
                if (node is StatementSyntax statement && node.Parent is BlockSyntax or SwitchSectionSyntax)
                {
                    inserts.Add((statement.SpanStart, Marker(Location(statement, file))));
                }

                if (node is BlockSyntax block && !block.CloseBraceToken.IsMissing)
                {
                    inserts.Add((block.CloseBraceToken.SpanStart, Marker(EndTag(block, file))));
                }
            }

            var text = source;
            foreach (var insert in inserts.OrderByDescending(insert => insert.Position))
            {
                text = text.Insert(insert.Position, insert.Text);
            }

            return text;
        }

        private static string Marker(string tag)
        {
            return "/*[[--@mw " + tag + "]]*/ ";
        }

        private static string Location(SyntaxNode node, int file)
        {
            var line = node.SyntaxTree.GetLineSpan(node.Span).StartLinePosition.Line + 1;
            return file + ":" + line;
        }

        // The statement a block is part of: the block itself when it is a statement of a block, else the nearest statement
        // around it (an if, a loop, the statement holding a lambda); none for a member's body or a lambda outside statements.
        private static string EndTag(BlockSyntax block, int file)
        {
            if (block.Parent is BlockSyntax or SwitchSectionSyntax)
            {
                return Location(block, file);
            }

            for (var node = block.Parent; node != null && node is not MemberDeclarationSyntax; node = node.Parent)
            {
                if (node is StatementSyntax and not BlockSyntax)
                {
                    return Location(node, file);
                }
            }

            return "-";
        }
    }
}
