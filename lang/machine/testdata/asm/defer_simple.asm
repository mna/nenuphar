### result: 2

program:
	names:
		G
	constants:
		int 1        # 0
		int 2        # 1
		string "result" # 2

# defer
# 	G.result = 2
# end
#	G.result = 1
function: top 3 0
	defers:
		6 12 1
	code:
		JMP  6       # goto constant 0
		PREDECLARED 0 # G
		CONSTANT 2    # result
		CONSTANT  1   # 2
		SETINDEX      # G.result = 2
		DEFEREXIT

		PREDECLARED 0 # G
		CONSTANT 2    # result
		CONSTANT 0    # 1
		SETINDEX      # G.result = 1
		NIL
		RUNDEFER
		RETURN

