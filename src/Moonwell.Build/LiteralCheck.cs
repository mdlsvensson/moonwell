using System.Globalization;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.Text;

namespace Moonwell.Build
{
    /// <summary>
    /// Finds the string literals of the map's C# that CSharp.lua copies into the script as written and Lua reads otherwise:
    /// <c>\e</c>, <c>\U</c>, <c>\x</c> without exactly two hex digits, <c>\0</c> before a digit, raw and UTF-8 string literals,
    /// and verbatim strings that start with a line end. Some make the game refuse the script, the others change the text.
    /// </summary>
    public static class LiteralCheck
    {
        /// <summary>Reports every such literal of the map's marked files as a build error, each with its file and line.</summary>
        /// <param name="projectFolder">The project folder.</param>
        /// <param name="files">The files to check, relative to the project folder.</param>
        /// <param name="symbols">The preprocessor symbols CSharp.lua compiles with.</param>
        /// <exception cref="BuildException">A literal Lua reads otherwise.</exception>
        public static void Check(string projectFolder, IEnumerable<string> files, IEnumerable<string> symbols)
        {
            var symbolList = symbols.ToList();
            var errors = files
                .SelectMany(file => Find(File.ReadAllText(Path.Combine(projectFolder, file)), file, symbolList))
                .ToList();
            if (errors.Count > 0)
            {
                throw new BuildException(string.Join(Environment.NewLine, errors));
            }
        }

        /// <summary>The literals of one file that Lua reads otherwise, as <c>file:line: what and what to write instead</c>.</summary>
        /// <param name="source">The file's text.</param>
        /// <param name="file">The file's name in the messages.</param>
        /// <param name="symbols">The preprocessor symbols CSharp.lua compiles with; inactive branches are not checked.</param>
        public static IEnumerable<string> Find(string source, string file, IEnumerable<string> symbols)
        {
            var options = CSharpParseOptions.Default.WithLanguageVersion(LanguageVersion.Latest).WithPreprocessorSymbols(symbols);
            var tree = CSharpSyntaxTree.ParseText(source, options);
            var errors = new List<string>();
            void Report(int position, string message)
            {
                var line = tree.GetLineSpan(new TextSpan(position, 0)).StartLinePosition.Line + 1;
                errors.Add(file + ":" + line + ": " + message);
            }

            foreach (var token in tree.GetRoot().DescendantTokens())
            {
                switch (token.Kind())
                {
                    case SyntaxKind.StringLiteralToken when token.Text.StartsWith('@'):
                        if (token.ValueText.StartsWith('\r'))
                        {
                            Report(token.SpanStart, VerbatimLineEnd);
                        }

                        break;
                    case SyntaxKind.StringLiteralToken:
                        Escapes(token.Text, token.SpanStart, Report);
                        break;
                    case SyntaxKind.InterpolatedStringTextToken:
                        Interpolated(token, Report);
                        break;
                    case SyntaxKind.SingleLineRawStringLiteralToken:
                    case SyntaxKind.MultiLineRawStringLiteralToken:
                        Report(token.SpanStart, "a raw string literal does not compile to Lua; use a regular or verbatim string");
                        break;
                    case SyntaxKind.Utf8StringLiteralToken:
                    case SyntaxKind.Utf8SingleLineRawStringLiteralToken:
                    case SyntaxKind.Utf8MultiLineRawStringLiteralToken:
                        Report(token.SpanStart, "a UTF-8 string literal (u8) does not compile to Lua");
                        break;
                }
            }

            return errors;
        }

        private const string VerbatimLineEnd = @"a verbatim string that starts with a line end loses it in Lua; write that line end as ""\r\n"" + ...";

        // A raw interpolated string has no escapes and compiles; a verbatim one has none either, but can start with a line end.
        private static void Interpolated(SyntaxToken token, Action<int, string> report)
        {
            if (token.Parent?.Parent is not InterpolatedStringExpressionSyntax expression)
            {
                return;
            }

            switch (expression.StringStartToken.Kind())
            {
                case SyntaxKind.InterpolatedStringStartToken:
                    Escapes(token.Text, token.SpanStart, report);
                    break;
                case SyntaxKind.InterpolatedVerbatimStringStartToken:
                    if (expression.Contents.FirstOrDefault() == token.Parent && token.ValueText.StartsWith('\r'))
                    {
                        report(token.SpanStart, VerbatimLineEnd);
                    }

                    break;
            }
        }

        // The escapes of a regular literal's text, as written: CSharp.lua copies them into a Lua string unchanged.
        private static void Escapes(string text, int start, Action<int, string> report)
        {
            for (var i = 0; i < text.Length - 1; i++)
            {
                if (text[i] != '\\')
                {
                    continue;
                }

                var kind = text[i + 1];
                if (kind == 'e')
                {
                    report(start + i, @"the escape \e does not compile to Lua; write \u001b");
                }
                else if (kind == 'U')
                {
                    var escape = text.Substring(i, Math.Min(10, text.Length - i));
                    report(start + i, $"the escape {escape} does not compile to Lua; write the character itself");
                }
                else if (kind == 'x')
                {
                    var digits = 0;
                    while (digits < 4 && i + 2 + digits < text.Length && Uri.IsHexDigit(text[i + 2 + digits]))
                    {
                        digits++;
                    }

                    if (digits != 2)
                    {
                        var hex = text.Substring(i + 2, digits);
                        var value = digits == 0 ? 0 : int.Parse(hex, NumberStyles.HexNumber, CultureInfo.InvariantCulture);
                        report(start + i, $@"the escape \x{hex} reads differently in Lua, which takes two hex digits; write \u{value:X4}");
                    }
                }
                else if (kind == '0' && i + 2 < text.Length && char.IsAsciiDigit(text[i + 2]))
                {
                    report(start + i, @"the escape \0 before a digit reads as a decimal escape in Lua; write \u0000");
                }

                i++;
            }
        }
    }
}
