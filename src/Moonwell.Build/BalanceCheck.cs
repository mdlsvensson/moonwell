namespace Moonwell.Build
{
    /// <summary>Checks that the standard objects a map builds on and uses exist under the game data option it plays with.</summary>
    public static class BalanceCheck
    {
        /// <summary>Fails for the first declared class whose standard object, or standard class the map's code uses, the game data option leaves out.</summary>
        /// <param name="declarations">What the map declares and uses.</param>
        /// <param name="standard">The standard data, which knows what each set of tables leaves out.</param>
        /// <param name="data">The map's game data option; its existence set decides what exists.</param>
        /// <exception cref="BuildException">An object the option leaves out does not exist in the game.</exception>
        public static void Run(DeclarationSet declarations, StandardData standard, GameData data)
        {
            foreach (var declared in declarations.Objects)
            {
                Check(standard, data, declared.ClassName, declared.StandardRoot);
            }

            foreach (var used in declarations.Used)
            {
                if (used.InObjects && used.StandardId != null)
                {
                    Check(standard, data, used.FullName, used.StandardId);
                }
            }
        }

        private static void Check(StandardData standard, GameData data, string className, string standardId)
        {
            if (standard.IsMissing(standardId, data.ExistenceSet))
            {
                throw new BuildException($"{className}: {standardId} does not exist under {data}");
            }
        }
    }
}
