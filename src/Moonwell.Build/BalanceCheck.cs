namespace Moonwell.Build
{
    /// <summary>Checks that the standard objects a map builds on and uses exist under the balance set it plays with.</summary>
    public static class BalanceCheck
    {
        /// <summary>Fails for the first declared class whose standard object, or standard class the map's code uses, the balance set leaves out.</summary>
        /// <param name="declarations">What the map declares and uses.</param>
        /// <param name="standard">The standard data, which knows what each balance set leaves out.</param>
        /// <param name="set">The balance set: null for the base tables.</param>
        /// <exception cref="BuildException">An object the set leaves out does not exist in the game.</exception>
        public static void Run(DeclarationSet declarations, StandardData standard, string? set)
        {
            foreach (var declared in declarations.Objects)
            {
                Check(standard, set, declared.ClassName, declared.StandardRoot);
            }

            foreach (var used in declarations.Used)
            {
                if (used.InObjects && used.StandardId != null)
                {
                    Check(standard, set, used.FullName, used.StandardId);
                }
            }
        }

        private static void Check(StandardData standard, string? set, string className, string standardId)
        {
            if (standard.IsMissing(standardId, set))
            {
                throw new BuildException($"{className}: {standardId} does not exist in {set ?? "base"}; its balance set leaves it out");
            }
        }
    }
}
