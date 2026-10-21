using System.Globalization;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.Operations;
using Microsoft.CodeAnalysis.Text;

namespace Moonwell.Build
{
    /// <summary>
    /// Writes every whole-number <c>float</c> and <c>double</c> constant of C# files with a decimal point, so that it reaches Lua
    /// as a float, not an integer (C18): CSharp.lua copies a literal's digits (<c>16777216f</c> gives <c>16777216</c>) and writes
    /// a constant it folds or inlines (<c>16777216f + 1f</c>, a const, a cast, an int stored in a float) by its value, without
    /// one. Each such constant becomes the one literal of its value (<c>16777216.0f</c>), which CSharp.lua copies as written; a
    /// whole-number default value a call leaves out, which CSharp.lua also writes by value, is written into the call, named.
    /// </summary>
    public static class FloatLiterals
    {
        /// <summary>Rewrites files of a folder in place.</summary>
        /// <param name="folder">The folder; every <c>*.cs</c> under it is compiled with the files, so names they use resolve.</param>
        /// <param name="files">The files to rewrite, relative to the folder.</param>
        /// <param name="references">The libraries the code references, beside the .NET libraries, which are added.</param>
        /// <param name="symbols">The preprocessor symbols CSharp.lua compiles with.</param>
        public static void Rewrite(string folder, IEnumerable<string> files, IEnumerable<string> references, IEnumerable<string> symbols)
        {
            var options = CSharpParseOptions.Default.WithLanguageVersion(LanguageVersion.Latest).WithPreprocessorSymbols(symbols);
            var trees = Directory.EnumerateFiles(folder, "*.cs", SearchOption.AllDirectories)
                .Select(path => CSharpSyntaxTree.ParseText(File.ReadAllText(path), options, Path.GetFullPath(path)))
                .ToList();
            var libraries = references.Concat(FrameworkLibraries())
                .Distinct(StringComparer.OrdinalIgnoreCase)
                .Select(path => MetadataReference.CreateFromFile(path));
            var compilation = CSharpCompilation.Create(
                "FloatLiterals",
                trees,
                libraries,
                new CSharpCompilationOptions(OutputKind.DynamicallyLinkedLibrary, allowUnsafe: true));

            var byPath = trees.ToDictionary(tree => tree.FilePath, StringComparer.OrdinalIgnoreCase);
            foreach (var file in files)
            {
                var tree = byPath[Path.GetFullPath(Path.Combine(folder, file))];
                var edits = Edits(tree.GetRoot(), compilation.GetSemanticModel(tree));
                if (edits.Count == 0)
                {
                    continue;
                }

                var text = tree.GetText().ToString();
                foreach (var edit in edits.OrderByDescending(edit => edit.Span.Start))
                {
                    text = text.Remove(edit.Span.Start, edit.Span.Length).Insert(edit.Span.Start, edit.Text);
                }

                File.WriteAllText(tree.FilePath, text);
            }
        }

        // The libraries of the .NET the build runs on, which is the one the map targets.
        private static IEnumerable<string> FrameworkLibraries()
        {
            var runtime = Path.GetDirectoryName(typeof(object).Assembly.Location)!;
            var trusted = (string?)AppContext.GetData("TRUSTED_PLATFORM_ASSEMBLIES") ?? string.Empty;
            return trusted.Split(Path.PathSeparator, StringSplitOptions.RemoveEmptyEntries)
                .Where(path => string.Equals(Path.GetDirectoryName(path), runtime, StringComparison.OrdinalIgnoreCase));
        }

        // A constant expression is rewritten whole, as CSharp.lua folds it whole; what is inside it is not looked at.
        private static List<(TextSpan Span, string Text)> Edits(SyntaxNode root, SemanticModel model)
        {
            bool IsConstant(SyntaxNode node) => node is ExpressionSyntax expression && model.GetConstantValue(expression).HasValue;

            var edits = new List<(TextSpan Span, string Text)>();
            foreach (var node in root.DescendantNodes(node => !IsConstant(node)))
            {
                if (!IsConstant(node))
                {
                    LeftOutArguments(node, model, edits);
                    continue;
                }

                var expression = (ExpressionSyntax)node;
                var literal = Literal(expression, model);
                if (literal != null)
                {
                    edits.Add((expression.Span, literal + LineEnds(expression)));
                }
            }

            return edits;
        }

        // The literal a constant expression is written as, or null when CSharp.lua already gives Lua a float, or the constant is
        // no float or double, or not a whole number.
        private static string? Literal(ExpressionSyntax expression, SemanticModel model)
        {
            var value = model.GetConstantValue(expression).Value;
            var info = model.GetTypeInfo(expression);
            bool isFloat;
            switch (value)
            {
                case float or double:
                    isFloat = value is float;
                    if (WrittenWithPoint(expression))
                    {
                        return null;
                    }

                    break;
                case sbyte or byte or short or ushort or int or uint or long or ulong:
                    var target = Unwrapped(info.ConvertedType);
                    if (target != SpecialType.System_Single && target != SpecialType.System_Double)
                    {
                        return null;
                    }

                    isFloat = target == SpecialType.System_Single;
                    value = isFloat ? Convert.ToSingle(value, CultureInfo.InvariantCulture) : Convert.ToDouble(value, CultureInfo.InvariantCulture);
                    break;
                default:
                    return null;
            }

            var literal = Text(value, isFloat);
            if (literal == null)
            {
                return null;
            }

            // A negative literal in place of a name or a cast could join the operator before it (a - -4.0f, a--4.0f).
            var negative = double.IsNegative(Convert.ToDouble(value, CultureInfo.InvariantCulture));
            return negative && !expression.IsKind(SyntaxKind.UnaryMinusExpression) ? "(" + literal + ")" : literal;
        }

        // A float or double literal with a point or an exponent, under brackets or a sign, which CSharp.lua copies as written.
        private static bool WrittenWithPoint(ExpressionSyntax expression)
        {
            while (expression is ParenthesizedExpressionSyntax or PrefixUnaryExpressionSyntax)
            {
                expression = expression is ParenthesizedExpressionSyntax parenthesized
                    ? parenthesized.Expression
                    : ((PrefixUnaryExpressionSyntax)expression).Operand;
            }

            return expression is LiteralExpressionSyntax literal
                && literal.Token.IsKind(SyntaxKind.NumericLiteralToken)
                && literal.Token.Value is float or double
                && literal.Token.Text.IndexOfAny(new[] { '.', 'e', 'E' }) >= 0;
        }

        // The C# literal of a whole-number float or double, with a point (16777216.0f) or an exponent; null for any other value.
        private static string? Text(object value, bool isFloat)
        {
            var number = Convert.ToDouble(value, CultureInfo.InvariantCulture);
            if (!double.IsFinite(number) || Math.Truncate(number) != number)
            {
                return null;
            }

            var text = number.ToString("R", CultureInfo.InvariantCulture);
            if (text.IndexOf('E') < 0)
            {
                text += ".0";
            }

            return isFloat ? text + "f" : text;
        }

        private static SpecialType Unwrapped(ITypeSymbol? type)
        {
            if (type is INamedTypeSymbol { OriginalDefinition.SpecialType: SpecialType.System_Nullable_T } nullable)
            {
                return nullable.TypeArguments[0].SpecialType;
            }

            return type?.SpecialType ?? SpecialType.None;
        }

        // The line ends of a constant written over several lines, kept so the lines after it do not move.
        private static string LineEnds(SyntaxNode node)
        {
            return new string(node.ToString().Where(character => character is '\r' or '\n').ToArray());
        }

        // A call that leaves out a parameter with a whole-number float or double default gets it written in, as a named argument.
        private static void LeftOutArguments(SyntaxNode node, SemanticModel model, List<(TextSpan Span, string Text)> edits)
        {
            if (node is not (InvocationExpressionSyntax or BaseObjectCreationExpressionSyntax or ConstructorInitializerSyntax or PrimaryConstructorBaseTypeSyntax or ElementAccessExpressionSyntax))
            {
                return;
            }

            var arguments = model.GetOperation(node) switch
            {
                IInvocationOperation invocation => invocation.Arguments,
                IObjectCreationOperation creation => creation.Arguments,
                IPropertyReferenceOperation indexer => indexer.Arguments,
                _ => default,
            };
            if (arguments.IsDefaultOrEmpty)
            {
                return;
            }

            var named = new List<string>();
            foreach (var argument in arguments.Where(argument => argument.ArgumentKind == ArgumentKind.DefaultValue && argument.Parameter != null))
            {
                var type = Unwrapped(argument.Parameter!.Type);
                var value = argument.Value.ConstantValue;
                if ((type != SpecialType.System_Single && type != SpecialType.System_Double) || !value.HasValue || value.Value == null)
                {
                    continue;
                }

                var isFloat = type == SpecialType.System_Single;
                var converted = isFloat ? Convert.ToSingle(value.Value, CultureInfo.InvariantCulture) : Convert.ToDouble(value.Value, CultureInfo.InvariantCulture);
                var literal = Text(converted, isFloat);
                if (literal != null)
                {
                    var name = argument.Parameter.Name;
                    var escaped = SyntaxFacts.GetKeywordKind(name) != SyntaxKind.None ? "@" + name : name;
                    named.Add(escaped + ": " + literal);
                }
            }

            if (named.Count == 0)
            {
                return;
            }

            var list = node switch
            {
                InvocationExpressionSyntax invocation => invocation.ArgumentList,
                BaseObjectCreationExpressionSyntax creation => creation.ArgumentList,
                ConstructorInitializerSyntax initializer => initializer.ArgumentList,
                PrimaryConstructorBaseTypeSyntax baseType => baseType.ArgumentList,
                ElementAccessExpressionSyntax access => (BaseArgumentListSyntax)access.ArgumentList,
                _ => null,
            };
            var text = string.Join(", ", named);
            if (list == null)
            {
                // new Thing { ... } has no brackets: they go after the type.
                var type = ((ObjectCreationExpressionSyntax)node).Type;
                edits.Add((new TextSpan(type.Span.End, 0), "(" + text + ")"));
                return;
            }

            var close = list is ArgumentListSyntax brackets ? brackets.CloseParenToken : ((BracketedArgumentListSyntax)list).CloseBracketToken;
            edits.Add((new TextSpan(close.SpanStart, 0), (list.Arguments.Count > 0 ? ", " : string.Empty) + text));
        }
    }
}
