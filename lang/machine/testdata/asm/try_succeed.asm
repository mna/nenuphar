### result: 3

program:
	names:
		G
	constants:
		int 1        # 0
		int 2        # 1
		string "result" # 2

function: top 3 0 # x = try 1 + 2
	locals:
		x  					# 0
	catches:
		3 5 1
	code:
		JMP  3     # goto try
		NIL
		CATCHJMP  6     # goto setlocal

		# try 1 + "a"
		CONSTANT 0 # 1
		CONSTANT 1 # "a"
		PLUS			 # 1 + "a"; throws

		# x = result
		SETLOCAL 0

		PREDECLARED 0 # G
		CONSTANT 2    # result
		LOCAL 0       # x
		SETINDEX      # G.result = x
		NIL
		RETURN

