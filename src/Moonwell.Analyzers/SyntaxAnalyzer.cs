using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.Diagnostics;
using Microsoft.CodeAnalysis.Operations;

namespace Moonwell.Analyzers;

/// <summary>
/// Rejects the C# syntax and library calls that CSharp.lua stops on, writes wrong or leaves to a Lua core that does not have
/// them, so the editor and dotnet build show it with its line (MW0017 to MW0034). Found by running about 770 checks
/// against .NET; each rule's description says what was seen. MW0019, MW0021 and MW0033 are gone: the fork's CSharp.lua
/// writes their forms right.
/// </summary>
[DiagnosticAnalyzer(LanguageNames.CSharp)]
public sealed class SyntaxAnalyzer : DiagnosticAnalyzer
{
    public override ImmutableArray<DiagnosticDescriptor> SupportedDiagnostics => Rules.Syntaxes;

    public override void Initialize(AnalysisContext context)
    {
        context.ConfigureGeneratedCodeAnalysis(GeneratedCodeAnalysisFlags.None);
        context.EnableConcurrentExecution();

        context.RegisterSyntaxNodeAction(SwitchArm, SyntaxKind.SwitchExpressionArm);
        context.RegisterSyntaxNodeAction(IsPattern, SyntaxKind.IsPatternExpression);
        context.RegisterSyntaxNodeAction(DeclaredFromElement, SyntaxKind.VariableDeclarator);
        context.RegisterSyntaxNodeAction(AssignedFromElement, SyntaxKind.SimpleAssignmentExpression);
        context.RegisterSyntaxNodeAction(Deconstruction, SyntaxKind.SimpleAssignmentExpression);
        context.RegisterSyntaxNodeAction(TupleSum, SyntaxKind.AddExpression);
        context.RegisterSyntaxNodeAction(TupleHole, SyntaxKind.Interpolation);
        context.RegisterSyntaxNodeAction(TupleArgument, SyntaxKind.Argument);
        context.RegisterSyntaxNodeAction(Operator, SyntaxKind.OperatorDeclaration);
        context.RegisterSyntaxNodeAction(InterfaceMember, SyntaxKind.MethodDeclaration, SyntaxKind.PropertyDeclaration, SyntaxKind.OperatorDeclaration);
        context.RegisterSyntaxNodeAction(DerivedRecord, SyntaxKind.RecordDeclaration);
        context.RegisterSyntaxNodeAction(TwoConstructions, SyntaxKind.ClassDeclaration, SyntaxKind.StructDeclaration);
        context.RegisterSyntaxNodeAction(WithAnonymous, SyntaxKind.WithExpression);
        context.RegisterSyntaxNodeAction(Range, SyntaxKind.RangeExpression);
        context.RegisterSyntaxNodeAction(IndexFromEnd, SyntaxKind.IndexExpression);
        context.RegisterSyntaxNodeAction(RefType, SyntaxKind.RefType);
        context.RegisterSyntaxNodeAction(RefLocal, SyntaxKind.RefExpression);
        context.RegisterSyntaxNodeAction(Params, SyntaxKind.Parameter);
        context.RegisterSyntaxNodeAction(StringCall, SyntaxKind.InvocationExpression);
        context.RegisterOperationAction(Boxed, OperationKind.Conversion);
        context.RegisterSyntaxNodeAction(CatchType, SyntaxKind.CatchDeclaration);
        context.RegisterSyntaxNodeAction(CatchFilter, SyntaxKind.CatchFilterClause);
        context.RegisterSyntaxNodeAction(Name, SyntaxKind.IdentifierName, SyntaxKind.GenericName);
        context.RegisterSyntaxNodeAction(Await, SyntaxKind.AwaitExpression);
        context.RegisterSyntaxNodeAction(Pointer, SyntaxKind.PointerType);
        context.RegisterSyntaxNodeAction(Checked, SyntaxKind.CheckedExpression, SyntaxKind.CheckedStatement);
        context.RegisterSyntaxNodeAction(EnumInvocation, SyntaxKind.InvocationExpression);
        context.RegisterSyntaxNodeAction(EnumSum, SyntaxKind.AddExpression);
        context.RegisterSyntaxNodeAction(EnumHole, SyntaxKind.Interpolation);
        context.RegisterSyntaxNodeAction(EnumCast, SyntaxKind.CastExpression);
        context.RegisterSyntaxNodeAction(Usings, SyntaxKind.LocalDeclarationStatement);
    }

    // ---- patterns (MW0017, MW0018) ----

    private static void SwitchArm(SyntaxNodeAnalysisContext context)
    {
        Walk(context, ((SwitchExpressionArmSyntax)context.Node).Pattern, arm: true, sub: false);
    }

    private static void IsPattern(SyntaxNodeAnalysisContext context)
    {
        Walk(context, ((IsPatternExpressionSyntax)context.Node).Pattern, arm: false, sub: false);
    }

    // arm: the pattern of a switch expression arm; sub: inside a property pattern.
    private static void Walk(SyntaxNodeAnalysisContext context, PatternSyntax pattern, bool arm, bool sub)
    {
        switch (pattern)
        {
            case BinaryPatternSyntax binary:
                Walk(context, binary.Left, arm, sub);
                Walk(context, binary.Right, arm, sub);
                break;
            case ParenthesizedPatternSyntax:
                Stops(context, pattern, "a pattern in parentheses");
                break;
            case UnaryPatternSyntax not:
                if (arm || sub)
                {
                    Stops(context, pattern, "not " + (arm ? "in a switch expression" : "in a property pattern"));
                }
                else
                {
                    Walk(context, not.Pattern, false, false);
                }

                break;
            case RelationalPatternSyntax when arm:
                Stops(context, pattern, "a relational pattern in a switch expression");
                break;
            case TypePatternSyntax when arm:
                Stops(context, pattern, "a type pattern without a name in a switch expression");
                break;
            case VarPatternSyntax when arm || sub:
                Stops(context, pattern, "var in " + (arm ? "a switch expression" : "a property pattern"));
                break;
            case DeclarationPatternSyntax when sub:
                Stops(context, pattern, "a name in a property pattern");
                break;
            case RecursivePatternSyntax recursive:
                if (recursive.PositionalPatternClause != null && !arm)
                {
                    context.ReportDiagnostic(Diagnostic.Create(Rules.PatternWrong, pattern.GetLocation(),
                        "A positional pattern in an is test is false or stops the compiler; use a property pattern ({ X: 1 }) or a switch expression"));
                }

                if (recursive.PropertyPatternClause != null)
                {
                    foreach (var subpattern in recursive.PropertyPatternClause.Subpatterns)
                    {
                        if (subpattern.ExpressionColon?.Expression is MemberAccessExpressionSyntax)
                        {
                            context.ReportDiagnostic(Diagnostic.Create(Rules.PatternWrong, subpattern.GetLocation(),
                                "An extended property pattern ({ A.B: 1 }) is always false; write { A: { B: 1 } }"));
                        }

                        Walk(context, subpattern.Pattern, false, true);
                    }
                }

                break;
            case ListPatternSyntax list:
                foreach (var item in list.Patterns)
                {
                    if (item is SlicePatternSyntax { Pattern: not null })
                    {
                        Stops(context, item, "a slice with a name in a list pattern (it needs Slice, which the game's Lua does not have)");
                    }
                }

                break;
        }
    }

    private static void Stops(SyntaxNodeAnalysisContext context, SyntaxNode node, string what)
    {
        context.ReportDiagnostic(Diagnostic.Create(Rules.PatternStops, node.GetLocation(), what));
    }

    // ---- MW0020 ----

    private static void DeclaredFromElement(SyntaxNodeAnalysisContext context)
    {
        var declarator = (VariableDeclaratorSyntax)context.Node;
        if (declarator.Initializer?.Value is ElementAccessExpressionSyntax access
            && context.SemanticModel.GetDeclaredSymbol(declarator, context.CancellationToken) is ILocalSymbol local)
        {
            CheckCopy(context, local, access, declarator);
        }
    }

    private static void AssignedFromElement(SyntaxNodeAnalysisContext context)
    {
        var assignment = (AssignmentExpressionSyntax)context.Node;
        if (assignment.Right is ElementAccessExpressionSyntax access && assignment.Left is IdentifierNameSyntax name
            && context.SemanticModel.GetSymbolInfo(name, context.CancellationToken).Symbol is ILocalSymbol local)
        {
            CheckCopy(context, local, access, assignment);
        }
    }

    private static void CheckCopy(SyntaxNodeAnalysisContext context, ILocalSymbol local, ElementAccessExpressionSyntax access, SyntaxNode scopeStart)
    {
        var type = context.SemanticModel.GetTypeInfo(access, context.CancellationToken).Type;
        if (type is not INamedTypeSymbol { TypeKind: TypeKind.Struct, SpecialType: SpecialType.None, IsReadOnly: false, OriginalDefinition.SpecialType: not SpecialType.System_Nullable_T })
        {
            return;
        }

        var scope = scopeStart.Ancestors().OfType<BlockSyntax>().FirstOrDefault();
        if (scope == null)
        {
            return;
        }

        foreach (var node in scope.DescendantNodes())
        {
            var target = node switch
            {
                AssignmentExpressionSyntax assignment => assignment.Left,
                PrefixUnaryExpressionSyntax prefix when prefix.IsKind(SyntaxKind.PreIncrementExpression) || prefix.IsKind(SyntaxKind.PreDecrementExpression) => prefix.Operand,
                PostfixUnaryExpressionSyntax postfix => postfix.Operand,
                _ => null,
            };

            if (target is MemberAccessExpressionSyntax { Expression: IdentifierNameSyntax receiver }
                && SymbolEqualityComparer.Default.Equals(context.SemanticModel.GetSymbolInfo(receiver, context.CancellationToken).Symbol, local))
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.StructCopy, access.GetLocation(), local.Name));
                return;
            }
        }
    }

    // ---- MW0022 ----

    private static void Deconstruction(SyntaxNodeAnalysisContext context)
    {
        var assignment = (AssignmentExpressionSyntax)context.Node;
        if (assignment.Left is DeclarationExpressionSyntax { Type.IsVar: true, Designation: ParenthesizedVariableDesignationSyntax } && assignment.Right is TupleExpressionSyntax)
        {
            Tuple(context, assignment, "var (a, b) = (...) gives the first name the whole tuple; write the types, (int a, int b) = (...)");
        }
        else if (assignment.Left is TupleExpressionSyntax left && left.Arguments.Any(a => a.Expression is ElementAccessExpressionSyntax))
        {
            Tuple(context, assignment, "assigning to a tuple of array elements is a Lua syntax error; use a temporary");
        }
    }

    private static void TupleSum(SyntaxNodeAnalysisContext context)
    {
        var add = (BinaryExpressionSyntax)context.Node;
        if (context.SemanticModel.GetTypeInfo(add, context.CancellationToken).Type?.SpecialType != SpecialType.System_String)
        {
            return;
        }

        foreach (var side in new[] { add.Left, add.Right })
        {
            if (IsTuple(context, side))
            {
                Tuple(context, side, "a tuple turned into text fails in ValueTuple.ToString; join its items");
            }
        }
    }

    private static void TupleHole(SyntaxNodeAnalysisContext context)
    {
        var hole = (InterpolationSyntax)context.Node;
        if (IsTuple(context, hole.Expression))
        {
            Tuple(context, hole, "a tuple turned into text fails in ValueTuple.ToString; write its items");
        }
    }

    private static void TupleArgument(SyntaxNodeAnalysisContext context)
    {
        var argument = (ArgumentSyntax)context.Node;
        var info = context.SemanticModel.GetTypeInfo(argument.Expression, context.CancellationToken);
        if (info.Type is INamedTypeSymbol { IsTupleType: true } && info.ConvertedType?.SpecialType == SpecialType.System_Object)
        {
            Tuple(context, argument, "a tuple given as an object fails in ValueTuple.ToString; write its items");
        }
        else if (argument.Parent?.Parent is InvocationExpressionSyntax { Expression: MemberAccessExpressionSyntax { Name.Identifier.Text: "ToString" } member }
            && IsTuple(context, member.Expression) && argument == ((InvocationExpressionSyntax)argument.Parent.Parent).ArgumentList.Arguments.FirstOrDefault())
        {
            Tuple(context, argument, "a tuple turned into text fails in ValueTuple.ToString; write its items");
        }
    }

    private static bool IsTuple(SyntaxNodeAnalysisContext context, ExpressionSyntax expression) =>
        context.SemanticModel.GetTypeInfo(expression, context.CancellationToken).Type is INamedTypeSymbol { IsTupleType: true };

    private static void Tuple(SyntaxNodeAnalysisContext context, SyntaxNode node, string message)
    {
        context.ReportDiagnostic(Diagnostic.Create(Rules.TupleForm, node.GetLocation(), char.ToUpperInvariant(message[0]) + message.Substring(1)));
    }

    // ---- MW0023 ----

    private static void Operator(SyntaxNodeAnalysisContext context)
    {
        var declaration = (OperatorDeclarationSyntax)context.Node;
        var token = declaration.OperatorToken.Kind();
        if (token is SyntaxKind.PlusPlusToken or SyntaxKind.MinusMinusToken or SyntaxKind.TrueKeyword or SyntaxKind.FalseKeyword)
        {
            Wrong(context, declaration, "A user-defined " + declaration.OperatorToken.Text + " is written as a + 1 or ignored; use a method");
        }
    }

    private static void InterfaceMember(SyntaxNodeAnalysisContext context)
    {
        var member = (MemberDeclarationSyntax)context.Node;
        if (member.Parent is InterfaceDeclarationSyntax && member is BaseMethodDeclarationSyntax or PropertyDeclarationSyntax)
        {
            var modifiers = member switch
            {
                BaseMethodDeclarationSyntax method => method.Modifiers,
                PropertyDeclarationSyntax property => property.Modifiers,
                _ => default,
            };

            if (modifiers.Any(SyntaxKind.StaticKeyword) && (modifiers.Any(SyntaxKind.AbstractKeyword) || modifiers.Any(SyntaxKind.VirtualKeyword)))
            {
                Wrong(context, member, "A static abstract or static virtual interface member called through a type parameter (T.Zero, a + b) fails in the game's Lua");
            }
        }
    }

    private static void DerivedRecord(SyntaxNodeAnalysisContext context)
    {
        var record = (RecordDeclarationSyntax)context.Node;
        if (record.BaseList?.Types.Any(t => t is PrimaryConstructorBaseTypeSyntax) == true)
        {
            Wrong(context, record, "A record that derives from a record: == is a Lua error and ToString leaves out the base's members");
        }
    }

    private static void TwoConstructions(SyntaxNodeAnalysisContext context)
    {
        var declaration = (TypeDeclarationSyntax)context.Node;
        if (context.SemanticModel.GetDeclaredSymbol(declaration, context.CancellationToken) is not INamedTypeSymbol symbol)
        {
            return;
        }

        var twice = symbol.AllInterfaces.Where(i => i.IsGenericType).GroupBy(i => i.OriginalDefinition, SymbolEqualityComparer.Default).FirstOrDefault(g => g.Count() > 1);
        if (twice != null)
        {
            Wrong(context, declaration.Identifier, "A class that implements " + twice.Key!.Name + " twice (with different type arguments) reaches the first implementation from both");
        }
    }

    private static void WithAnonymous(SyntaxNodeAnalysisContext context)
    {
        var with = (WithExpressionSyntax)context.Node;
        if (context.SemanticModel.GetTypeInfo(with.Expression, context.CancellationToken).Type is { IsAnonymousType: true })
        {
            Wrong(context, with, "with on an anonymous type is a Lua error");
        }
    }

    private static void Wrong(SyntaxNodeAnalysisContext context, SyntaxNodeOrToken node, string message)
    {
        context.ReportDiagnostic(Diagnostic.Create(Rules.FormWrong, node.GetLocation(), message));
    }

    // ---- MW0024 ----

    private static void Range(SyntaxNodeAnalysisContext context)
    {
        context.ReportDiagnostic(Diagnostic.Create(Rules.RangeIndex, context.Node.GetLocation(), context.Node.ToString()));
    }

    private static void IndexFromEnd(SyntaxNodeAnalysisContext context)
    {
        // a[^1] on an array works.
        if (context.Node.Parent is ArgumentSyntax { Parent: BracketedArgumentListSyntax { Parent: ElementAccessExpressionSyntax access } }
            && context.SemanticModel.GetTypeInfo(access.Expression, context.CancellationToken).Type is IArrayTypeSymbol)
        {
            return;
        }

        context.ReportDiagnostic(Diagnostic.Create(Rules.RangeIndex, context.Node.GetLocation(), context.Node.ToString()));
    }

    // ---- MW0025 ----

    private static void RefType(SyntaxNodeAnalysisContext context)
    {
        var type = (RefTypeSyntax)context.Node;
        var returned = type.Parent switch
        {
            MethodDeclarationSyntax method => method.ReturnType == type,
            LocalFunctionStatementSyntax local => local.ReturnType == type,
            PropertyDeclarationSyntax property => property.Type == type,
            IndexerDeclarationSyntax indexer => indexer.Type == type,
            _ => false,
        };

        if (returned)
        {
            context.ReportDiagnostic(Diagnostic.Create(Rules.RefForm, type.GetLocation(), "A ref return is a copy in the game's Lua: writes through it are lost"));
        }
    }

    private static void RefLocal(SyntaxNodeAnalysisContext context)
    {
        var reference = (RefExpressionSyntax)context.Node;
        if (reference.Parent is EqualsValueClauseSyntax && reference.Expression is ElementAccessExpressionSyntax or InvocationExpressionSyntax)
        {
            context.ReportDiagnostic(Diagnostic.Create(Rules.RefForm, reference.GetLocation(),
                reference.Expression is InvocationExpressionSyntax
                    ? "A ref local of a call's result is a copy in the game's Lua: writes through it are lost"
                    : "A ref local of an element is a Lua syntax error; index the array each time"));
        }
    }

    // ---- MW0026 ----

    private static void Params(SyntaxNodeAnalysisContext context)
    {
        var parameter = (ParameterSyntax)context.Node;
        if (!parameter.Modifiers.Any(SyntaxKind.ParamsKeyword))
        {
            return;
        }

        if (parameter.Parent?.Parent is ParenthesizedLambdaExpressionSyntax)
        {
            context.ReportDiagnostic(Diagnostic.Create(Rules.ParamsForm, parameter.GetLocation(), "params in a lambda is a Lua error; take an array"));
        }
        else if (parameter.Type != null && context.SemanticModel.GetTypeInfo(parameter.Type, context.CancellationToken).Type is { } type && type is not IArrayTypeSymbol)
        {
            context.ReportDiagnostic(Diagnostic.Create(Rules.ParamsForm, parameter.GetLocation(), "params " + type.Name + " (C# 13) stops the compiler; use params with an array"));
        }
    }

    // ---- MW0027 ----

    private static void StringCall(SyntaxNodeAnalysisContext context)
    {
        var invocation = (InvocationExpressionSyntax)context.Node;
        if (context.SemanticModel.GetSymbolInfo(invocation, context.CancellationToken).Symbol is not IMethodSymbol method)
        {
            return;
        }

        var reduced = method.ReducedFrom ?? method;
        if (reduced.ContainingType?.SpecialType == SpecialType.System_String)
        {
            var first = reduced.Parameters.FirstOrDefault()?.Type;
            if (reduced.Name == "Split" && first?.SpecialType == SpecialType.System_String)
            {
                Report(context, invocation, "Split(string) runs out of memory in the game's Lua; split on a char");
            }
            else if (reduced.Name is "StartsWith" or "EndsWith" && first?.SpecialType == SpecialType.System_Char)
            {
                Report(context, invocation, reduced.Name + "(char) fails in the game's Lua; use a string");
            }
            else if (reduced.Name is "IndexOf" or "StartsWith" or "EndsWith" && first?.SpecialType == SpecialType.System_String
                && invocation.ArgumentList.Arguments.Any(a => IgnoresCase(context, a.Expression)))
            {
                Report(context, invocation, reduced.Name + " with an ignore-case comparison gives the wrong answer or an error; lower-case both strings");
            }
        }
        else if (reduced.ContainingType is { Name: "Enumerable", ContainingNamespace: { Name: "Linq" } } && reduced.Name == "Reverse"
            && invocation.Expression is MemberAccessExpressionSyntax access
            && context.SemanticModel.GetTypeInfo(access.Expression, context.CancellationToken).Type?.SpecialType == SpecialType.System_String)
        {
            Report(context, invocation, "Reverse() on a string fails in the game's Lua; reverse a char array with Array.Reverse");
        }
    }

    // An argument that is a constant StringComparison with IgnoreCase in its name: CurrentCultureIgnoreCase 1, InvariantCultureIgnoreCase 3, OrdinalIgnoreCase 5.
    private static bool IgnoresCase(SyntaxNodeAnalysisContext context, ExpressionSyntax expression)
    {
        var type = context.SemanticModel.GetTypeInfo(expression, context.CancellationToken).Type;
        if (type is not { Name: "StringComparison", TypeKind: TypeKind.Enum })
        {
            return false;
        }

        var value = context.SemanticModel.GetConstantValue(expression, context.CancellationToken);
        return value.HasValue && value.Value is int number && number is 1 or 3 or 5;
    }

    private static void Report(SyntaxNodeAnalysisContext context, SyntaxNode node, string message)
    {
        context.ReportDiagnostic(Diagnostic.Create(Rules.StringCall, node.GetLocation(), node.ToString(), message));
    }

    // ---- MW0028 ----

    private static void Boxed(OperationAnalysisContext context)
    {
        var conversion = (IConversionOperation)context.Operation;
        if (conversion.Type?.SpecialType != SpecialType.System_Object || conversion.Operand.Type is not { } from)
        {
            return;
        }

        string? why = null;
        if (from.SpecialType == SpecialType.System_Char)
        {
            why = "is shown as its number (97 for 'a') when given as an object in the game's Lua; call ToString() on it";
        }
        else if (from.TypeKind == TypeKind.Enum && !IsChoice(from))
        {
            why = "is shown as its number when given as an object in the game's Lua; call ToString() on it";
        }

        if (why == null)
        {
            return;
        }

        // Only the library methods that make the text: an argument of a method that is not the map's own.
        var up = conversion.Parent;
        while (up is IArrayInitializerOperation or IArrayCreationOperation)
        {
            up = up.Parent;
        }

        if (up is IArgumentOperation { Parent: IInvocationOperation invocation }
            && invocation.TargetMethod.Locations.All(l => !l.IsInSource)
            && invocation.TargetMethod.Name is "WriteLine" or "Write" or "Format" or "Concat" or "Append" or "AppendFormat" or "Join")
        {
            context.ReportDiagnostic(Diagnostic.Create(Rules.BoxedValue, conversion.Operand.Syntax.GetLocation(), conversion.Operand.Syntax.ToString(), why));
        }
    }

    private static bool IsChoice(ITypeSymbol type) =>
        type.ContainingNamespace is { Name: "Objects", ContainingNamespace: { Name: "Moonwell", ContainingNamespace.IsGlobalNamespace: true } };

    // ---- MW0029 ----

    private static void CatchType(SyntaxNodeAnalysisContext context)
    {
        var declaration = (CatchDeclarationSyntax)context.Node;
        if (context.SemanticModel.GetTypeInfo(declaration.Type, context.CancellationToken).Type is { Name: "NullReferenceException" or "IndexOutOfRangeException", ContainingNamespace: { Name: "System", ContainingNamespace.IsGlobalNamespace: true } } type)
        {
            context.ReportDiagnostic(Diagnostic.Create(Rules.CatchForm, declaration.Type.GetLocation(),
                "catch (" + type.Name + ") may never catch in the game's Lua: a null dereference or an array index out of range is a plain Exception there; catch (Exception), or test for null and the length first"));
        }
    }

    private static void CatchFilter(SyntaxNodeAnalysisContext context)
    {
        var filter = (CatchFilterClauseSyntax)context.Node;
        foreach (var pattern in filter.FilterExpression.DescendantNodesAndSelf().OfType<DeclarationPatternSyntax>())
        {
            context.ReportDiagnostic(Diagnostic.Create(Rules.CatchForm, pattern.GetLocation(),
                "A pattern variable in a catch filter is nil in the game's Lua; test with is and cast in the block"));
        }
    }

    // ---- MW0030 ----

    private const string Missing = "is not in the game's Lua core; it fails when it runs";

    private static readonly ImmutableHashSet<string> MissingTypes = ImmutableHashSet.Create(
        "System.Range", "System.Index", "System.StringComparer", "System.Threading.Monitor", "System.Threading.Lock",
        "System.Threading.Thread", "System.Threading.Tasks.Task", "System.Threading.Tasks.Task`1", "System.Collections.BitArray",
        "System.Numerics.BitOperations", "System.HashCode", "System.Guid", "System.FormattableString");

    private static readonly ImmutableDictionary<string, string> Members = new Dictionary<string, string>
    {
        ["System.Linq.Enumerable.Append"] = Missing,
        ["System.Linq.Enumerable.Prepend"] = Missing,
        ["System.Linq.Enumerable.SkipLast"] = Missing,
        ["System.Linq.Enumerable.TakeLast"] = Missing,
        ["System.Linq.Enumerable.ToLookup"] = Missing,
        ["System.Linq.Enumerable.Empty"] = Missing,
        ["System.Linq.Enumerable.Zip"] = Missing,
        ["System.Linq.Enumerable.Chunk"] = Missing,
        ["System.Linq.Enumerable.ToHashSet"] = Missing,
        ["System.Linq.Enumerable.DefaultIfEmpty"] = "gives 0 or stops where .NET gives the default item",
        ["System.Collections.Generic.List`1.ConvertAll"] = Missing,
        ["System.Span`1.ToArray"] = Missing,
        ["System.Span`1.Fill"] = Missing,
        ["System.Span`1.Clear"] = Missing,
        ["System.Text.StringBuilder.Replace"] = Missing,
        ["System.Text.StringBuilder.Insert"] = Missing,
        ["System.Text.StringBuilder.Equals"] = Missing,
        ["System.Text.StringBuilder.AppendJoin"] = Missing,
        ["System.Exception.Source"] = Missing,
        ["System.Delegate.Target"] = Missing,
        ["System.Delegate.Method"] = Missing,
        ["System.Char.IsAscii"] = Missing,
        ["System.Char.ToUpperInvariant"] = Missing,
        ["System.Environment.NewLine"] = Missing + "; write \"\\n\"",
        ["System.Enum.GetUnderlyingType"] = Missing,
        ["System.Nullable.GetUnderlyingType"] = Missing,
        ["System.String.Create"] = Missing,
        ["System.String.Normalize"] = Missing,
        ["System.String.Intern"] = Missing,
        ["System.String.EnumerateRunes"] = Missing,
        ["System.Collections.Generic.Queue`1.TryPeek"] = Missing,
        ["System.Collections.Generic.Stack`1.TryPop"] = Missing,
        ["System.Collections.Generic.Stack`1.TryPeek"] = Missing,
        ["System.Collections.Generic.CollectionExtensions.GetValueOrDefault"] = Missing + "; use TryGetValue",
        ["System.Convert.ToHexString"] = Missing,
        ["System.Int32.RotateLeft"] = Missing,
        ["System.Int32.RotateRight"] = Missing,
        ["System.UInt32.RotateLeft"] = Missing,
        ["System.UInt32.RotateRight"] = Missing,
        ["System.Collections.Generic.HashSet`1.SetEquals"] = "says False for equal sets in the game's Lua; compare Count and Contains",
        ["System.Math.DivRem"] = "gives its quotient as a float (3.0) in the game's Lua; use / and %",
    }.ToImmutableDictionary();

    private static void Name(SyntaxNodeAnalysisContext context)
    {
        var name = (SimpleNameSyntax)context.Node;
        if (name is IdentifierNameSyntax { IsVar: true })
        {
            return;
        }

        var symbol = context.SemanticModel.GetSymbolInfo(name, context.CancellationToken).Symbol;
        if (symbol == null || symbol.Locations.Any(l => l.IsInSource))
        {
            return;
        }

        if (symbol is INamedTypeSymbol type)
        {
            if (MissingTypes.Contains(Full(type.OriginalDefinition)))
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.LuaMember, name.GetLocation(), name.Identifier.Text, Missing));
            }

            return;
        }

        var method = symbol as IMethodSymbol;
        var original = method != null ? (method.ReducedFrom ?? method).OriginalDefinition : symbol.OriginalDefinition;
        if (original.ContainingType == null)
        {
            return;
        }

        var key = Full(original.ContainingType) + "." + original.Name;
        if (Members.TryGetValue(key, out var why))
        {
            context.ReportDiagnostic(Diagnostic.Create(Rules.LuaMember, name.GetLocation(), name.Identifier.Text, why));
            return;
        }

        if (method == null || name.Parent is not (MemberAccessExpressionSyntax or InvocationExpressionSyntax or MemberBindingExpressionSyntax))
        {
            return;
        }

        var reduced = method.ReducedFrom ?? method;
        if (key == "System.Array.Sort" && reduced.Parameters.Length == 2 && reduced.Parameters[1].Type is IArrayTypeSymbol or { Name: "Array" })
        {
            context.ReportDiagnostic(Diagnostic.Create(Rules.LuaMember, name.GetLocation(), name.Identifier.Text, "with a keys array and an items array fails in the game's Lua"));
        }
        else if (key == "System.Collections.Generic.Dictionary`2.Remove" && reduced.Parameters.Length == 2)
        {
            context.ReportDiagnostic(Diagnostic.Create(Rules.LuaMember, name.GetLocation(), name.Identifier.Text, "leaves the out value nil in the game's Lua; read it with TryGetValue first"));
        }
        else if (key is "System.Linq.Enumerable.ToList" or "System.Linq.Enumerable.ToArray"
            && method.TypeArguments.FirstOrDefault() is INamedTypeSymbol { Name: "KeyValuePair", ContainingNamespace.Name: "Generic" })
        {
            context.ReportDiagnostic(Diagnostic.Create(Rules.LuaMember, name.GetLocation(), name.Identifier.Text, "of a Dictionary gives pairs without their Key in the game's Lua; copy with new Dictionary<,>(d), or Select the key and value"));
        }
    }

    private static string Full(INamedTypeSymbol type) =>
        (type.ContainingType != null ? Full(type.ContainingType) + "+" : type.ContainingNamespace.ToDisplayString() + ".") + type.MetadataName;

    private static void Await(SyntaxNodeAnalysisContext context)
    {
        context.ReportDiagnostic(Diagnostic.Create(Rules.LuaMember, context.Node.GetLocation(), "await", "needs Task, which the game's Lua does not have; there are no threads. Use a Timer"));
    }

    private static void Pointer(SyntaxNodeAnalysisContext context)
    {
        context.ReportDiagnostic(Diagnostic.Create(Rules.LuaMember, context.Node.GetLocation(), context.Node.ToString(), "is a pointer, which CSharp.lua cannot compile"));
    }

    // ---- MW0031 ----

    private static void Checked(SyntaxNodeAnalysisContext context)
    {
        // A checked cast does throw; only checked arithmetic on the integer types is missed.
        var arithmetic = context.Node.DescendantNodes().Any(node =>
            node is BinaryExpressionSyntax or PrefixUnaryExpressionSyntax or PostfixUnaryExpressionSyntax or AssignmentExpressionSyntax
            && !node.IsKind(SyntaxKind.SimpleAssignmentExpression) && !node.IsKind(SyntaxKind.LogicalNotExpression)
            && IsInteger(context.SemanticModel.GetTypeInfo(node, context.CancellationToken).Type));
        if (arithmetic)
        {
            context.ReportDiagnostic(Diagnostic.Create(Rules.CheckedForm, context.Node.GetLocation()));
        }
    }

    private static bool IsInteger(ITypeSymbol? type) => type?.SpecialType is SpecialType.System_Int16 or SpecialType.System_Int32 or SpecialType.System_Int64
        or SpecialType.System_UInt16 or SpecialType.System_UInt32 or SpecialType.System_UInt64 or SpecialType.System_Byte or SpecialType.System_SByte;

    // ---- MW0032 ----

    private static void EnumInvocation(SyntaxNodeAnalysisContext context)
    {
        var invocation = (InvocationExpressionSyntax)context.Node;
        if (context.SemanticModel.GetSymbolInfo(invocation, context.CancellationToken).Symbol is not IMethodSymbol method)
        {
            return;
        }

        if (method.IsStatic && method.ContainingType?.SpecialType == SpecialType.System_Enum && method.Name is "Parse" or "GetValues" or "GetNames")
        {
            var choice = method.TypeArguments.Any(IsChoice)
                || invocation.ArgumentList.Arguments.Any(a => a.Expression is TypeOfExpressionSyntax typeOf
                    && context.SemanticModel.GetTypeInfo(typeOf.Type, context.CancellationToken).Type is { } t && IsChoice(t));
            // Enum.GetNames(...).Length counts; it does not depend on the order.
            var counted = invocation.Parent is MemberAccessExpressionSyntax { Name.Identifier.Text: "Length" or "Count" };
            if (!choice && !(counted && method.Name != "Parse"))
            {
                Enum(context, invocation, method.Name == "Parse"
                    ? "Enum.Parse gives the number, not the enum value in the game's Lua; compare strings or use a switch"
                    : "Enum." + method.Name + " comes in no fixed order in the game's Lua; list the values yourself");
            }

            return;
        }

        if (method is { Name: "Join" or "Concat", ContainingType.SpecialType: SpecialType.System_String })
        {
            foreach (var argument in invocation.ArgumentList.Arguments)
            {
                var argumentType = context.SemanticModel.GetTypeInfo(argument.Expression, context.CancellationToken).Type;
                var element = argumentType switch
                {
                    IArrayTypeSymbol array => array.ElementType,
                    INamedTypeSymbol { IsGenericType: true, TypeArguments.Length: 1 } generic when generic.Name is "IEnumerable" or "List" or "IList" or "ICollection" or "IReadOnlyList" => generic.TypeArguments[0],
                    _ => null,
                };

                if (element is { TypeKind: TypeKind.Enum } && !IsChoice(element))
                {
                    context.ReportDiagnostic(Diagnostic.Create(Rules.BoxedValue, argument.Expression.GetLocation(), argument.Expression.ToString(),
                        "joins the enum's numbers, not its names, in the game's Lua; Select(x => x.ToString()) first"));
                }
            }
        }

        if (method is { Name: "ToString", IsStatic: false, Parameters.Length: > 0 } && invocation.Expression is MemberAccessExpressionSyntax access
            && context.SemanticModel.GetTypeInfo(access.Expression, context.CancellationToken).Type is { TypeKind: TypeKind.Enum } format && !IsChoice(format))
        {
            Enum(context, invocation, "ToString(format) on an enum gives the name whatever the format; cast to int for the number");
            return;
        }

        if (method is { Name: "ToString", IsStatic: false, Parameters.Length: 0 } && invocation.Expression is MemberAccessExpressionSyntax receiver)
        {
            FlagsText(context, receiver.Expression, invocation);
        }
    }

    private static void EnumSum(SyntaxNodeAnalysisContext context)
    {
        var add = (BinaryExpressionSyntax)context.Node;
        if (context.SemanticModel.GetTypeInfo(add, context.CancellationToken).Type?.SpecialType != SpecialType.System_String)
        {
            return;
        }

        foreach (var side in new[] { add.Left, add.Right })
        {
            FlagsText(context, side, side);
        }
    }

    private static void EnumHole(SyntaxNodeAnalysisContext context)
    {
        var hole = (InterpolationSyntax)context.Node;
        FlagsText(context, hole.Expression, hole);
    }

    private static void FlagsText(SyntaxNodeAnalysisContext context, ExpressionSyntax expression, SyntaxNode reported)
    {
        if (context.SemanticModel.GetTypeInfo(expression, context.CancellationToken).Type is { TypeKind: TypeKind.Enum } type && !IsChoice(type)
            && type.GetAttributes().Any(a => a.AttributeClass is { Name: "FlagsAttribute", ContainingNamespace.Name: "System" }))
        {
            Enum(context, reported, "A [Flags] value turned into text prints its number in the game's Lua when it is a combination; build the text from HasFlag");
        }
    }

    // (Color)5 + " x": a constant cast to an enum, joined to text, stops the compiler.
    private static void EnumCast(SyntaxNodeAnalysisContext context)
    {
        var cast = (CastExpressionSyntax)context.Node;
        if (context.SemanticModel.GetTypeInfo(cast, context.CancellationToken).Type is not { TypeKind: TypeKind.Enum } type || IsChoice(type)
            || !context.SemanticModel.GetConstantValue(cast.Expression, context.CancellationToken).HasValue)
        {
            return;
        }

        var joined = cast.Parent is BinaryExpressionSyntax add && add.IsKind(SyntaxKind.AddExpression)
            && context.SemanticModel.GetTypeInfo(add, context.CancellationToken).Type?.SpecialType == SpecialType.System_String;
        if (joined || cast.Parent is InterpolationSyntax)
        {
            context.ReportDiagnostic(Diagnostic.Create(Rules.EnumText, cast.GetLocation(), "A constant cast to an enum, joined to text, stops the compiler; write the member's name, or put the cast in a variable"));
        }
    }

    private static void Enum(SyntaxNodeAnalysisContext context, SyntaxNode node, string message)
    {
        context.ReportDiagnostic(Diagnostic.Create(Rules.EnumText, node.GetLocation(), message));
    }

    // ---- MW0034 ----

    private static void Usings(SyntaxNodeAnalysisContext context)
    {
        var declaration = (LocalDeclarationStatementSyntax)context.Node;
        if (declaration.UsingKeyword.IsKind(SyntaxKind.None) || declaration.Parent is not BlockSyntax block)
        {
            return;
        }

        if (block.Statements.TakeWhile(s => s != declaration).OfType<LocalDeclarationStatementSyntax>().Any(s => !s.UsingKeyword.IsKind(SyntaxKind.None)))
        {
            context.ReportDiagnostic(Diagnostic.Create(Rules.UsingOrder, declaration.GetLocation()));
        }
    }
}
