using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.Diagnostics;
using Microsoft.CodeAnalysis.Operations;

namespace Moonwell.Analyzers;

/// <summary>Rejects the multiplayer mistakes of decision 13 in a map project.</summary>
[DiagnosticAnalyzer(LanguageNames.CSharp)]
public sealed class SyncAnalyzer : DiagnosticAnalyzer
{
    public override ImmutableArray<DiagnosticDescriptor> SupportedDiagnostics => Rules.All;

    public override void Initialize(AnalysisContext context)
    {
        context.ConfigureGeneratedCodeAnalysis(GeneratedCodeAnalysisFlags.None);
        context.EnableConcurrentExecution();
        context.RegisterCompilationStartAction(start =>
        {
            var checker = new Checker(start.Compilation);
            start.RegisterOperationAction(checker.Call,
                OperationKind.Invocation, OperationKind.PropertyReference, OperationKind.MethodReference, OperationKind.ObjectCreation);
            start.RegisterOperationAction(checker.Loop, OperationKind.Loop);
            start.RegisterOperationAction(checker.Conversion, OperationKind.Conversion);
            start.RegisterOperationAction(checker.Literal, OperationKind.Literal);
            start.RegisterOperationAction(checker.Declarator, OperationKind.VariableDeclarator);
            start.RegisterSymbolAction(checker.Declaration, SymbolKind.Field, SymbolKind.Property, SymbolKind.Method, SymbolKind.NamedType);
        });
    }

    /// <summary>Why a call may not run on one machine: the path from the call to what it reaches, and what that does.</summary>
    private sealed class Finding
    {
        public Finding(string path, string what)
        {
            Path = path;
            What = what;
        }

        public string Path { get; }

        public string What { get; }

        public string Message(string name) =>
            Path == name ? $"'{name}' {What}" : $"'{name}' {What} (through {Path})";
    }

    /// <summary>The checks of one compilation, with the symbols they compare against.</summary>
    private sealed class Checker
    {
        private readonly Compilation compilation;
        private readonly INamedTypeSymbol? natives;
        private readonly IMethodSymbol? localFor;
        private readonly IAssemblySymbol? runtime;
        /// <summary>Moonwell's packages that the map references, checked as the runtime is.</summary>
        private readonly IAssemblySymbol[] packages;
        private readonly INamedTypeSymbol? createsHandle;
        private readonly INamedTypeSymbol? changesState;
        private readonly INamedTypeSymbol? machineLocal;
        private readonly INamedTypeSymbol? desyncs;
        private readonly INamedTypeSymbol? localSafe;
        private readonly INamedTypeSymbol? syncUnchecked;
        private readonly INamedTypeSymbol?[] hashTypes;
        private readonly INamedTypeSymbol? enumerable;
        private readonly INamedTypeSymbol? enumerableOfT;

        /// <summary>What each followed map method reaches; a method that reaches nothing rejected maps to null.</summary>
        private readonly ConcurrentDictionary<IMethodSymbol, Finding?> followed = new(SymbolEqualityComparer.Default);

        public Checker(Compilation compilation)
        {
            this.compilation = compilation;
            natives = compilation.GetTypeByMetadataName("Moonwell.Natives");
            var local = compilation.GetTypeByMetadataName("Moonwell.Local");
            localFor = local?.GetMembers("For").OfType<IMethodSymbol>().FirstOrDefault();
            runtime = local?.ContainingAssembly;
            // Each package is found by one of its types.
            packages = new[] { "Moonwell.DamageSystem", "Moonwell.Vector2" }
                .Select(name => compilation.GetTypeByMetadataName(name)?.ContainingAssembly)
                .Where(assembly => assembly != null)
                .Select(assembly => assembly!)
                .ToArray();
            createsHandle = compilation.GetTypeByMetadataName("Moonwell.CreatesHandleAttribute");
            changesState = compilation.GetTypeByMetadataName("Moonwell.ChangesStateAttribute");
            machineLocal = compilation.GetTypeByMetadataName("Moonwell.MachineLocalAttribute");
            desyncs = compilation.GetTypeByMetadataName("Moonwell.DesyncsAttribute");
            localSafe = compilation.GetTypeByMetadataName("Moonwell.LocalSafeAttribute");
            syncUnchecked = compilation.GetTypeByMetadataName("Moonwell.SyncUncheckedAttribute");
            hashTypes = new[]
            {
                compilation.GetTypeByMetadataName("System.Collections.Generic.Dictionary`2"),
                compilation.GetTypeByMetadataName("System.Collections.Generic.Dictionary`2+KeyCollection"),
                compilation.GetTypeByMetadataName("System.Collections.Generic.Dictionary`2+ValueCollection"),
                compilation.GetTypeByMetadataName("System.Collections.Generic.HashSet`1"),
            };
            enumerable = compilation.GetTypeByMetadataName("System.Collections.IEnumerable");
            enumerableOfT = compilation.GetTypeByMetadataName("System.Collections.Generic.IEnumerable`1");
        }

        public void Call(OperationAnalysisContext context)
        {
            var operation = context.Operation;
            var (member, write) = Target(operation);
            if (member == null || Silenced(operation, context.ContainingSymbol))
            {
                return;
            }

            var location = operation.Syntax.GetLocation();
            if (IsNative(member) && member.Name is "GetHandleId" or "GetHandleIdBJ")
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.HandleId, location));
            }

            if (member.Name == "GetEnumerator" && IsHash(member.ContainingType))
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.HashLoop, location, member.ContainingType.Name));
            }

            if (Has(member, desyncs))
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.Desyncs, location, Name(member)));
            }
            else if (IsLocal(operation, context.ContainingSymbol))
            {
                var finding = Rejected(member, write, new HashSet<IMethodSymbol>(SymbolEqualityComparer.Default));
                if (finding != null)
                {
                    context.ReportDiagnostic(Diagnostic.Create(Rules.StateInLocal, location, finding.Message(Name(member))));
                }
            }
            else if (IsMachineLocal(member, write) && !DeclaredName(operation, member, write) && !InNameOf(operation))
            {
                // 0.2.2's Name read the declared name outside the constructor too; the message says where it is now.
                var declared = member is IPropertySymbol { Name: "Name" } && IsRuntime(member) && member.ContainingType.Name == "Unit" ? "; the declared name is Default.Name" : string.Empty;
                context.ReportDiagnostic(Diagnostic.Create(Rules.LocalOutside, location, Name(member), declared));
            }
        }

        /// <summary>
        /// Whether a read of a unit's name (not a hero's proper name, which has no declared value) is the declared one, the same on every machine: in a declared class's
        /// constructor on the object itself, or on a default object (<c>….Default</c> or <c>Unit.Default&lt;T&gt;()</c>).
        /// A copy of a default object in a variable is not known to be one.
        /// </summary>
        private bool DeclaredName(IOperation operation, ISymbol member, bool write)
        {
            if (write || member is not IPropertySymbol { Name: "Name" } property || !IsRuntime(property)
                || operation is not IPropertyReferenceOperation { Instance: { } instance })
            {
                return false;
            }

            if (instance is IInstanceReferenceOperation)
            {
                return InDeclaredConstructor(operation);
            }

            return instance switch
            {
                IPropertyReferenceOperation { Property: { Name: "Default", IsStatic: false } defaults } => IsRuntimeObject(defaults.ContainingType),
                IInvocationOperation { TargetMethod: { Name: "Default", IsGenericMethod: true } method } => IsRuntime(method),
                _ => false,
            };
        }

        // nameof(Name) names the member without reading it.
        private static bool InNameOf(IOperation operation)
        {
            for (var node = operation.Syntax.Parent; node != null && node is not StatementSyntax and not MemberDeclarationSyntax; node = node.Parent)
            {
                if (node is InvocationExpressionSyntax { Expression: IdentifierNameSyntax { Identifier.ValueText: "nameof" } } invocation
                    && operation.SemanticModel!.GetSymbolInfo(invocation).Symbol == null)
                {
                    return true;
                }
            }

            return false;
        }

        // A unit or item class of Moonwell or of Moonwell.Objects, whose Default is the one of the runtime's Unit or Item.
        private bool IsRuntimeObject(INamedTypeSymbol? type)
        {
            for (; type != null; type = type.BaseType)
            {
                if (IsRuntime(type))
                {
                    return true;
                }
            }

            return false;
        }

        private bool IsRuntime(ISymbol member) =>
            runtime != null && SymbolEqualityComparer.Default.Equals(member.ContainingAssembly, runtime);

        /// <summary>Whether an operation is in the body of an instance constructor of a declared class, and not in a lambda or local function there, which may run later.</summary>
        private static bool InDeclaredConstructor(IOperation operation)
        {
            for (var node = operation.Syntax; node != null; node = node.Parent)
            {
                switch (node)
                {
                    case AnonymousFunctionExpressionSyntax or LocalFunctionStatementSyntax:
                        return false;
                    case ConstructorDeclarationSyntax constructor:
                        return !constructor.Modifiers.Any(SyntaxKind.StaticKeyword)
                            && operation.SemanticModel!.GetDeclaredSymbol(constructor)?.ContainingType is { } type
                            && DeclaredAnalyzer.IsDeclared(type);
                    case MemberDeclarationSyntax:
                        return false;
                }
            }

            return false;
        }

        public void Loop(OperationAnalysisContext context)
        {
            if (context.Operation is IForEachLoopOperation loop && !Silenced(loop, context.ContainingSymbol)
                && Unwrap(loop.Collection).Type is { } type && IsHash(type))
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.HashLoop, loop.Collection.Syntax.GetLocation(), type.Name));
            }
        }

        public void Conversion(OperationAnalysisContext context)
        {
            var conversion = (IConversionOperation)context.Operation;
            if (Silenced(conversion, context.ContainingSymbol))
            {
                return;
            }

            if (!conversion.IsImplicit && conversion.Type is { } target && Banned(target) is { } banned)
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.NumberType, conversion.Syntax.GetLocation(), banned, Why(banned)));
            }

            if (conversion.Operand.Type is { } source && IsHash(source) && IsEnumerable(conversion.Type)
                && !(conversion.Parent is IForEachLoopOperation loop && loop.Collection == conversion))
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.HashLoop, conversion.Operand.Syntax.GetLocation(), source.Name));
            }
        }

        public void Literal(OperationAnalysisContext context)
        {
            var literal = context.Operation;
            if (literal.Type is { } type && Banned(type) is { } banned && !Silenced(literal, context.ContainingSymbol))
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.NumberType, literal.Syntax.GetLocation(), banned, Why(banned)));
            }
        }

        public void Declarator(OperationAnalysisContext context)
        {
            var declarator = (IVariableDeclaratorOperation)context.Operation;
            if (Banned(declarator.Symbol.Type) is { } banned && !Silenced(declarator, context.ContainingSymbol))
            {
                var location = declarator.Symbol.Locations.FirstOrDefault() ?? declarator.Syntax.GetLocation();
                context.ReportDiagnostic(Diagnostic.Create(Rules.NumberType, location, banned, Why(banned)));
            }
        }

        public void Declaration(SymbolAnalysisContext context)
        {
            var symbol = context.Symbol;
            if (symbol.IsImplicitlyDeclared || Has(symbol, syncUnchecked))
            {
                return;
            }

            var types = new List<ITypeSymbol>();
            switch (symbol)
            {
                case IFieldSymbol field:
                    types.Add(field.Type);
                    break;
                case IPropertySymbol property:
                    types.Add(property.Type);
                    types.AddRange(property.Parameters.Select(p => p.Type));
                    break;
                case IMethodSymbol method when method.AssociatedSymbol == null:
                    types.Add(method.ReturnType);
                    types.AddRange(method.Parameters.Select(p => p.Type));
                    break;
                case INamedTypeSymbol { EnumUnderlyingType: { } underlying }:
                    types.Add(underlying);
                    break;
            }

            var banned = types.Select(Banned).FirstOrDefault(b => b != null);
            if (banned != null && symbol.Locations.FirstOrDefault() is { } location && location.IsInSource)
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.NumberType, location, banned, Why(banned)));
            }
        }

        /// <summary>The member an operation calls, and whether it writes a property.</summary>
        private static (ISymbol? Member, bool Write) Target(IOperation operation) => operation switch
        {
            IInvocationOperation invocation => (invocation.TargetMethod, false),
            IPropertyReferenceOperation property => (property.Property, IsWritten(property)),
            IMethodReferenceOperation reference => (reference.Method, false),
            IObjectCreationOperation creation => (creation.Constructor, false),
            _ => (null, false),
        };

        private static bool IsWritten(IOperation operation) => operation.Parent switch
        {
            IAssignmentOperation assignment => assignment.Target == operation,
            IIncrementOrDecrementOperation => true,
            _ => false,
        };

        /// <summary>Why a member may not be called on one machine; null when it may.</summary>
        private Finding? Rejected(ISymbol member, bool write, HashSet<IMethodSymbol> visiting)
        {
            var name = Name(member);
            if (IsNative(member))
            {
                if (member is IPropertySymbol)
                {
                    return write ? new Finding(name, "is set, which changes game state") : null;
                }

                if (Has(member, createsHandle))
                {
                    return new Finding(name, "creates a handle");
                }

                return Has(member, changesState) ? new Finding(name, "changes game state") : null;
            }

            if ((runtime != null && SymbolEqualityComparer.Default.Equals(member.ContainingAssembly, runtime))
                || packages.Any(package => SymbolEqualityComparer.Default.Equals(member.ContainingAssembly, package)))
            {
                // Making a Moonwell object changes nothing in the game until it is added or started.
                if (member is IMethodSymbol { MethodKind: MethodKind.Constructor } || IsLocalSafe(member, write) || IsMachineLocal(member, write))
                {
                    return null;
                }

                return new Finding(name, "may change game state (it is not marked [LocalSafe])");
            }

            if (!SymbolEqualityComparer.Default.Equals(member.ContainingAssembly, compilation.Assembly)
                || Has(member, syncUnchecked) || IsMachineLocal(member, write))
            {
                return null;
            }

            var methods = member switch
            {
                IMethodSymbol method => new[] { method },
                IPropertySymbol property => new[] { write ? property.SetMethod : property.GetMethod },
                _ => new IMethodSymbol?[0],
            };
            foreach (var method in methods)
            {
                if (method != null && Follow(method, visiting) is { } inner)
                {
                    return new Finding(name + " > " + inner.Path, inner.What);
                }
            }

            return null;
        }

        /// <summary>What a method of the map reaches that may not run on one machine.</summary>
        private Finding? Follow(IMethodSymbol method, HashSet<IMethodSymbol> visiting)
        {
            method = method.OriginalDefinition;
            if (followed.TryGetValue(method, out var known))
            {
                return known;
            }

            if (!visiting.Add(method))
            {
                return null;
            }

            Finding? found = null;
            foreach (var reference in method.DeclaringSyntaxReferences)
            {
                var syntax = reference.GetSyntax();
                var body = compilation.GetSemanticModel(syntax.SyntaxTree).GetOperation(syntax);
                if (body == null)
                {
                    continue;
                }

                foreach (var inner in body.DescendantsAndSelf())
                {
                    var (member, write) = Target(inner);
                    if (member != null && (found = Rejected(member, write, visiting)) != null)
                    {
                        break;
                    }
                }

                if (found != null)
                {
                    break;
                }
            }

            visiting.Remove(method);
            followed[method] = found;
            return found;
        }

        /// <summary>Whether an operation runs on one machine: inside Local.For's action, or in a [MachineLocal] method.</summary>
        private bool IsLocal(IOperation operation, ISymbol container)
        {
            var model = operation.SemanticModel!;
            for (var node = operation.Syntax; node != null; node = node.Parent)
            {
                if (node is ArgumentSyntax argument && IsLocalForAction(argument, model))
                {
                    return true;
                }

                if (node is LocalFunctionStatementSyntax or AnonymousFunctionExpressionSyntax
                    && DeclaredBy(node, model) is { } function && Has(function, machineLocal))
                {
                    return true;
                }

                if (node is MemberDeclarationSyntax)
                {
                    break;
                }
            }

            return Has(container, machineLocal);
        }

        private bool IsLocalForAction(ArgumentSyntax argument, SemanticModel model)
        {
            if (localFor == null || argument.Parent is not ArgumentListSyntax list || list.Parent is not InvocationExpressionSyntax invocation)
            {
                return false;
            }

            var isAction = argument.NameColon != null
                ? argument.NameColon.Name.Identifier.ValueText == localFor.Parameters[1].Name
                : list.Arguments.IndexOf(argument) == 1;
            return isAction && SymbolEqualityComparer.Default.Equals(model.GetSymbolInfo(invocation).Symbol?.OriginalDefinition, localFor);
        }

        /// <summary>Whether [SyncUnchecked] is on the member, local function or lambda around an operation.</summary>
        private bool Silenced(IOperation operation, ISymbol container)
        {
            if (syncUnchecked == null)
            {
                return false;
            }

            var model = operation.SemanticModel!;
            for (var node = operation.Syntax; node != null && node is not MemberDeclarationSyntax; node = node.Parent)
            {
                if (node is LocalFunctionStatementSyntax or AnonymousFunctionExpressionSyntax
                    && DeclaredBy(node, model) is { } function && Has(function, syncUnchecked))
                {
                    return true;
                }
            }

            return Has(container, syncUnchecked);
        }

        private static ISymbol? DeclaredBy(SyntaxNode node, SemanticModel model) =>
            node is LocalFunctionStatementSyntax ? model.GetDeclaredSymbol(node) : model.GetSymbolInfo(node).Symbol;

        private bool IsNative(ISymbol member) => natives != null && SymbolEqualityComparer.Default.Equals(member.ContainingType, natives);

        /// <summary>Whether a runtime member is marked local-safe: itself (a property's getter only), or its class.</summary>
        private bool IsLocalSafe(ISymbol member, bool write)
        {
            if (member is IMethodSymbol { AssociatedSymbol: IPropertySymbol property })
            {
                write = SymbolEqualityComparer.Default.Equals(property.SetMethod, member);
                member = property;
            }

            if (!write && Has(member, localSafe, false))
            {
                return true;
            }

            for (var type = member.ContainingType; type != null; type = type.ContainingType)
            {
                if (Has(type.OriginalDefinition, localSafe, false))
                {
                    return true;
                }
            }

            return false;
        }

        /// <summary>Whether a member is local to one machine: marked [MachineLocal] itself, or a property whose getter is, for a read only.</summary>
        private bool IsMachineLocal(ISymbol member, bool write) =>
            Has(member, machineLocal) || (!write && member is IPropertySymbol property && Has(property.GetMethod, machineLocal, false));

        /// <summary>Whether a symbol carries an attribute; an accessor also takes its property's.</summary>
        private static bool Has(ISymbol? symbol, INamedTypeSymbol? attribute, bool withProperty = true)
        {
            if (symbol == null || attribute == null)
            {
                return false;
            }

            if (symbol.GetAttributes().Any(a => SymbolEqualityComparer.Default.Equals(a.AttributeClass, attribute)))
            {
                return true;
            }

            return withProperty && symbol is IMethodSymbol { AssociatedSymbol: { } associated } && Has(associated, attribute, false);
        }

        private bool IsHash(ITypeSymbol type)
        {
            for (var current = type as INamedTypeSymbol; current != null; current = current.BaseType)
            {
                if (hashTypes.Any(h => h != null && SymbolEqualityComparer.Default.Equals(current.OriginalDefinition, h)))
                {
                    return true;
                }
            }

            return false;
        }

        private bool IsEnumerable(ITypeSymbol? type) =>
            type != null && (SymbolEqualityComparer.Default.Equals(type, enumerable)
                || SymbolEqualityComparer.Default.Equals(type.OriginalDefinition, enumerableOfT));

        private static IOperation Unwrap(IOperation operation)
        {
            while (operation is IConversionOperation { IsImplicit: true } conversion)
            {
                operation = conversion.Operand;
            }

            return operation;
        }

        /// <summary>The number type the game cannot carry that a type is or holds; null for none.</summary>
        private static string? Banned(ITypeSymbol type)
        {
            switch (type.SpecialType)
            {
                case SpecialType.System_Int64:
                    return "long";
                case SpecialType.System_UInt64:
                    return "ulong";
                case SpecialType.System_UInt32:
                    return "uint";
                case SpecialType.System_Double:
                    return "double";
                case SpecialType.System_Decimal:
                    return "decimal";
            }

            return type switch
            {
                IArrayTypeSymbol array => Banned(array.ElementType),
                INamedTypeSymbol named => named.TypeArguments.Select(Banned).FirstOrDefault(b => b != null),
                _ => null,
            };
        }

        private static string Why(string banned) => banned switch
        {
            "double" => "is a single-precision float in the game's Lua; use float",
            "uint" => "arithmetic becomes a float in the game's Lua; use int",
            "decimal" => "is an ordinary number in the game's Lua, so 0.1m + 0.2m is not 0.3m; use int or float",
            _ => "is a 32-bit integer in the game's Lua; use int",
        };

        private static string Name(ISymbol member) => member switch
        {
            IMethodSymbol { MethodKind: MethodKind.Constructor } constructor => constructor.ContainingType.Name,
            _ when member.ContainingType is { Name: "Natives" } => member.Name,
            _ => member.ContainingType != null ? member.ContainingType.Name + "." + member.Name : member.Name,
        };
    }
}
